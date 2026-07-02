// Package repo provides PostgreSQL-backed Treasury persistence.
package repo

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/treasury/internal/domain"
)

// Querier is the subset of pgx behaviour shared by *pgxpool.Pool and pgx.Tx.
// Repos depend on this so the same query methods can run either directly on the
// pool or inside a transaction (see TxStore).
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var (
	ErrNotFound            = errors.New("record not found")
	ErrInsufficientBalance = errors.New("insufficient reserve balance")
)

// ReserveRepo persists reserve account state.
type ReserveRepo interface {
	List(ctx context.Context) ([]domain.ReserveAccount, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.ReserveAccount, error)
	ByTypeAndCurrency(ctx context.Context, accountType domain.AccountType, currency, arena string) (domain.ReserveAccount, error)
	Credit(ctx context.Context, id uuid.UUID, amountWei *big.Int) error
	Debit(ctx context.Context, id uuid.UUID, amountWei *big.Int) error
}

// SettlementRepo persists settlement records.
type SettlementRepo interface {
	Create(ctx context.Context, s domain.Settlement) error
	List(ctx context.Context, limit, offset int) ([]domain.Settlement, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Settlement, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SettlementStatus, settledAt *time.Time) error
}

// ReconciliationRepo persists reconciliation logs.
type ReconciliationRepo interface {
	Create(ctx context.Context, r domain.ReconciliationLog) error
	ListByAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.ReconciliationLog, error)
}

// ── PostgreSQL implementations ─────────────────────────────────────────────

type pgReserveRepo struct{ db Querier }

func NewPGReserveRepo(db Querier) ReserveRepo { return &pgReserveRepo{db: db} }

func (r *pgReserveRepo) List(ctx context.Context) ([]domain.ReserveAccount, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, account_type, balance_wei, currency, arena, created_at, updated_at
		 FROM treasury.reserve_accounts ORDER BY currency, account_type`)
	if err != nil {
		return nil, fmt.Errorf("list reserve accounts: %w", err)
	}
	defer rows.Close()
	return scanReserveAccounts(rows)
}

func (r *pgReserveRepo) ByID(ctx context.Context, id uuid.UUID) (domain.ReserveAccount, error) {
	return r.scanOne(ctx,
		`SELECT id, account_type, balance_wei, currency, arena, created_at, updated_at
		 FROM treasury.reserve_accounts WHERE id = $1`, id)
}

func (r *pgReserveRepo) ByTypeAndCurrency(ctx context.Context, accountType domain.AccountType, currency, arena string) (domain.ReserveAccount, error) {
	return r.scanOne(ctx,
		`SELECT id, account_type, balance_wei, currency, arena, created_at, updated_at
		 FROM treasury.reserve_accounts
		 WHERE account_type = $1 AND currency = $2 AND arena = $3`,
		string(accountType), currency, arena)
}

func (r *pgReserveRepo) Credit(ctx context.Context, id uuid.UUID, amountWei *big.Int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE treasury.reserve_accounts
		 SET balance_wei = balance_wei + $2, updated_at = now()
		 WHERE id = $1`,
		id, amountWei.String())
	if err != nil {
		return fmt.Errorf("credit reserve account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgReserveRepo) Debit(ctx context.Context, id uuid.UUID, amountWei *big.Int) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE treasury.reserve_accounts
		 SET balance_wei = balance_wei - $2, updated_at = now()
		 WHERE id = $1 AND balance_wei >= $2`,
		id, amountWei.String())
	if err != nil {
		return fmt.Errorf("debit reserve account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either not found or balance was insufficient — check which.
		var exists bool
		_ = r.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM treasury.reserve_accounts WHERE id = $1)`, id,
		).Scan(&exists)
		if !exists {
			return ErrNotFound
		}
		return ErrInsufficientBalance
	}
	return nil
}

func (r *pgReserveRepo) scanOne(ctx context.Context, q string, args ...any) (domain.ReserveAccount, error) {
	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return domain.ReserveAccount{}, fmt.Errorf("query reserve account: %w", err)
	}
	defer rows.Close()
	accs, err := scanReserveAccounts(rows)
	if err != nil {
		return domain.ReserveAccount{}, err
	}
	if len(accs) == 0 {
		return domain.ReserveAccount{}, ErrNotFound
	}
	return accs[0], nil
}

func scanReserveAccounts(rows pgx.Rows) ([]domain.ReserveAccount, error) {
	var out []domain.ReserveAccount
	for rows.Next() {
		var (
			acc      domain.ReserveAccount
			balStr   string
			accType  string
		)
		if err := rows.Scan(
			&acc.ID, &accType, &balStr,
			&acc.Currency, &acc.Arena,
			&acc.CreatedAt, &acc.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan reserve account: %w", err)
		}
		acc.AccountType = domain.AccountType(accType)
		bal, err := parseBigInt(balStr)
		if err != nil {
			return nil, fmt.Errorf("parse balance for account %s: %w", acc.ID, err)
		}
		acc.BalanceWei = bal
		out = append(out, acc)
	}
	return out, rows.Err()
}

// ── Settlement repo ────────────────────────────────────────────────────────

type pgSettlementRepo struct{ db Querier }

func NewPGSettlementRepo(db Querier) SettlementRepo { return &pgSettlementRepo{db: db} }

func (r *pgSettlementRepo) Create(ctx context.Context, s domain.Settlement) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO treasury.settlements
			(id, settlement_type, account_id, amount_wei, reference_id, reference_type, tx_hash, status, settled_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		s.ID, string(s.SettlementType), s.AccountID, s.AmountWei.String(),
		s.ReferenceID, s.ReferenceType, s.TxHash, string(s.Status),
		s.SettledAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert settlement: %w", err)
	}
	return nil
}

func (r *pgSettlementRepo) List(ctx context.Context, limit, offset int) ([]domain.Settlement, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, settlement_type, account_id, amount_wei, reference_id, reference_type,
		        tx_hash, status, settled_at, created_at
		 FROM treasury.settlements
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list settlements: %w", err)
	}
	defer rows.Close()
	return scanSettlements(rows)
}

func (r *pgSettlementRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Settlement, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, settlement_type, account_id, amount_wei, reference_id, reference_type,
		        tx_hash, status, settled_at, created_at
		 FROM treasury.settlements WHERE id = $1`, id)
	if err != nil {
		return domain.Settlement{}, fmt.Errorf("query settlement: %w", err)
	}
	defer rows.Close()
	ss, err := scanSettlements(rows)
	if err != nil {
		return domain.Settlement{}, err
	}
	if len(ss) == 0 {
		return domain.Settlement{}, ErrNotFound
	}
	return ss[0], nil
}

func (r *pgSettlementRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SettlementStatus, settledAt *time.Time) error {
	tag, err := r.db.Exec(ctx,
		`UPDATE treasury.settlements SET status = $2, settled_at = $3 WHERE id = $1`,
		id, string(status), settledAt)
	if err != nil {
		return fmt.Errorf("update settlement status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSettlements(rows pgx.Rows) ([]domain.Settlement, error) {
	var out []domain.Settlement
	for rows.Next() {
		var (
			s         domain.Settlement
			amtStr    string
			sType     string
			sStatus   string
		)
		if err := rows.Scan(
			&s.ID, &sType, &s.AccountID, &amtStr,
			&s.ReferenceID, &s.ReferenceType,
			&s.TxHash, &sStatus, &s.SettledAt, &s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan settlement: %w", err)
		}
		s.SettlementType = domain.SettlementType(sType)
		s.Status = domain.SettlementStatus(sStatus)
		amt, err := parseBigInt(amtStr)
		if err != nil {
			return nil, fmt.Errorf("parse amount for settlement %s: %w", s.ID, err)
		}
		s.AmountWei = amt
		out = append(out, s)
	}
	return out, rows.Err()
}

// ── Reconciliation repo ────────────────────────────────────────────────────

type pgReconciliationRepo struct{ db Querier }

func NewPGReconciliationRepo(db Querier) ReconciliationRepo {
	return &pgReconciliationRepo{db: db}
}

func (r *pgReconciliationRepo) Create(ctx context.Context, log domain.ReconciliationLog) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO treasury.reconciliation_logs
			(id, account_id, expected_balance_wei, actual_balance_wei, status, reconciled_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		log.ID, log.AccountID,
		log.ExpectedBalanceWei.String(), log.ActualBalanceWei.String(),
		string(log.Status), log.ReconciledAt,
	)
	if err != nil {
		return fmt.Errorf("insert reconciliation log: %w", err)
	}
	return nil
}

func (r *pgReconciliationRepo) ListByAccount(ctx context.Context, accountID uuid.UUID, limit int) ([]domain.ReconciliationLog, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, account_id, expected_balance_wei, actual_balance_wei, discrepancy_wei, status, reconciled_at
		 FROM treasury.reconciliation_logs
		 WHERE account_id = $1
		 ORDER BY reconciled_at DESC
		 LIMIT $2`, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("list reconciliation logs: %w", err)
	}
	defer rows.Close()

	var out []domain.ReconciliationLog
	for rows.Next() {
		var (
			l        domain.ReconciliationLog
			expStr   string
			actStr   string
			discStr  string
			status   string
		)
		if err := rows.Scan(
			&l.ID, &l.AccountID,
			&expStr, &actStr, &discStr,
			&status, &l.ReconciledAt,
		); err != nil {
			return nil, fmt.Errorf("scan reconciliation log: %w", err)
		}
		exp, err := parseBigInt(expStr)
		if err != nil {
			return nil, fmt.Errorf("parse expected balance for log %s: %w", l.ID, err)
		}
		act, err := parseBigInt(actStr)
		if err != nil {
			return nil, fmt.Errorf("parse actual balance for log %s: %w", l.ID, err)
		}
		disc, err := parseBigInt(discStr)
		if err != nil {
			return nil, fmt.Errorf("parse discrepancy for log %s: %w", l.ID, err)
		}
		l.ExpectedBalanceWei = exp
		l.ActualBalanceWei = act
		l.DiscrepancyWei = disc
		l.Status = domain.ReconciliationStatus(status)
		out = append(out, l)
	}
	return out, rows.Err()
}

// ── transactional store ───────────────────────────────────────────────────

// TxStore runs multi-statement operations atomically against the pool.
type TxStore struct{ pool *pgxpool.Pool }

// NewTxStore constructs a transactional store.
func NewTxStore(pool *pgxpool.Pool) *TxStore { return &TxStore{pool: pool} }

// ApplyManualSettlement atomically adjusts the reserve balance AND records the
// settlement in a single transaction, so the two can never diverge (e.g. a
// debited balance with no settlement record). On any failure the whole
// operation is rolled back.
func (s *TxStore) ApplyManualSettlement(ctx context.Context, accountID uuid.UUID, credit bool, settlement domain.Settlement) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin settlement tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	reserves := NewPGReserveRepo(tx)
	if credit {
		err = reserves.Credit(ctx, accountID, settlement.AmountWei)
	} else {
		err = reserves.Debit(ctx, accountID, settlement.AmountWei)
	}
	if err != nil {
		return err
	}

	if err := NewPGSettlementRepo(tx).Create(ctx, settlement); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ErrAlreadyProcessed is returned by ApplySettlementIdempotent when a settlement
// with the same (reference_id, reference_type) already exists — i.e. the source
// event was redelivered. Callers should treat it as a successful no-op (ack).
var ErrAlreadyProcessed = errors.New("treasury: settlement already processed")

// ApplySettlementIdempotent atomically records the settlement AND adjusts the reserve
// balance in one transaction, keyed on (reference_id, reference_type). The settlement
// is inserted FIRST: if a row with the same idempotency key already exists the unique
// constraint rejects it, the transaction rolls back, and ErrAlreadyProcessed is
// returned WITHOUT crediting/debiting — so a duplicate event delivery can never
// double-count the reserve.
func (s *TxStore) ApplySettlementIdempotent(ctx context.Context, accountID uuid.UUID, credit bool, settlement domain.Settlement) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin settlement tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := NewPGSettlementRepo(tx).Create(ctx, settlement); err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyProcessed
		}
		return err
	}

	reserves := NewPGReserveRepo(tx)
	if credit {
		err = reserves.Credit(ctx, accountID, settlement.AmountWei)
	} else {
		err = reserves.Debit(ctx, accountID, settlement.AmountWei)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// isUniqueViolation reports whether err is a PostgreSQL unique-constraint violation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ── helpers ────────────────────────────────────────────────────────────────

// parseBigInt parses a base-10 integer string, returning an error rather than
// silently yielding zero on malformed input — critical on balance/amount paths
// where a corrupted NUMERIC value must never be read as a 0 balance.
func parseBigInt(s string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid integer value %q", s)
	}
	return n, nil
}
