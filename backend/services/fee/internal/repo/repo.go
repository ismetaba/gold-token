// Package repo provides PostgreSQL-backed Fee Management persistence.
package repo

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/fee/internal/domain"
)

var ErrNotFound = errors.New("record not found")

// ScheduleRepo manages fee schedule tiers.
type ScheduleRepo interface {
	List(ctx context.Context, operationType, arena string) ([]domain.FeeSchedule, error)
	ListAll(ctx context.Context) ([]domain.FeeSchedule, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.FeeSchedule, error)
	Update(ctx context.Context, id uuid.UUID, feeBPS int, minFeeWei *big.Int, active bool) error
	FindTier(ctx context.Context, operationType, arena string, amountWei *big.Int) (domain.FeeSchedule, error)
}

// LedgerRepo manages fee ledger entries.
type LedgerRepo interface {
	Create(ctx context.Context, e domain.LedgerEntry) error
	List(ctx context.Context, limit, offset int) ([]domain.LedgerEntry, error)
}

// ── PostgreSQL implementations ─────────────────────────────────────────────

type pgScheduleRepo struct{ pool *pgxpool.Pool }

func NewPGScheduleRepo(pool *pgxpool.Pool) ScheduleRepo { return &pgScheduleRepo{pool: pool} }

func (r *pgScheduleRepo) ListAll(ctx context.Context) ([]domain.FeeSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, operation_type, arena, tier_min_grams_wei, tier_max_grams_wei,
		        fee_bps, min_fee_wei, active, created_at, updated_at
		 FROM fee.schedules ORDER BY operation_type, arena, tier_min_grams_wei`)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()
	return scanSchedules(rows)
}

func (r *pgScheduleRepo) List(ctx context.Context, operationType, arena string) ([]domain.FeeSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, operation_type, arena, tier_min_grams_wei, tier_max_grams_wei,
		        fee_bps, min_fee_wei, active, created_at, updated_at
		 FROM fee.schedules
		 WHERE operation_type = $1 AND (arena = $2 OR arena = 'global') AND active = true
		 ORDER BY tier_min_grams_wei`, operationType, arena)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()
	return scanSchedules(rows)
}

func (r *pgScheduleRepo) ByID(ctx context.Context, id uuid.UUID) (domain.FeeSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, operation_type, arena, tier_min_grams_wei, tier_max_grams_wei,
		        fee_bps, min_fee_wei, active, created_at, updated_at
		 FROM fee.schedules WHERE id = $1`, id)
	if err != nil {
		return domain.FeeSchedule{}, fmt.Errorf("query schedule: %w", err)
	}
	defer rows.Close()
	ss, err := scanSchedules(rows)
	if err != nil {
		return domain.FeeSchedule{}, err
	}
	if len(ss) == 0 {
		return domain.FeeSchedule{}, ErrNotFound
	}
	return ss[0], nil
}

func (r *pgScheduleRepo) Update(ctx context.Context, id uuid.UUID, feeBPS int, minFeeWei *big.Int, active bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE fee.schedules SET fee_bps = $2, min_fee_wei = $3, active = $4, updated_at = now() WHERE id = $1`,
		id, feeBPS, minFeeWei.String(), active)
	if err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgScheduleRepo) FindTier(ctx context.Context, operationType, arena string, amountWei *big.Int) (domain.FeeSchedule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, operation_type, arena, tier_min_grams_wei, tier_max_grams_wei,
		        fee_bps, min_fee_wei, active, created_at, updated_at
		 FROM fee.schedules
		 WHERE operation_type = $1 AND (arena = $2 OR arena = 'global') AND active = true
		   AND tier_min_grams_wei <= $3
		   AND (tier_max_grams_wei IS NULL OR tier_max_grams_wei > $3)
		 ORDER BY CASE WHEN arena = $2 THEN 0 ELSE 1 END
		 LIMIT 1`,
		operationType, arena, amountWei.String())
	if err != nil {
		return domain.FeeSchedule{}, fmt.Errorf("find tier: %w", err)
	}
	defer rows.Close()
	ss, err := scanSchedules(rows)
	if err != nil {
		return domain.FeeSchedule{}, err
	}
	if len(ss) == 0 {
		return domain.FeeSchedule{}, ErrNotFound
	}
	return ss[0], nil
}

func scanSchedules(rows pgx.Rows) ([]domain.FeeSchedule, error) {
	var out []domain.FeeSchedule
	for rows.Next() {
		var (
			s        domain.FeeSchedule
			minStr   string
			maxStr   *string
			feeStr   string
		)
		if err := rows.Scan(
			&s.ID, &s.Name, &s.OperationType, &s.Arena,
			&minStr, &maxStr, &s.FeeBPS, &feeStr,
			&s.Active, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan schedule: %w", err)
		}
		s.TierMinGramsWei = mustParseBigInt(minStr)
		if maxStr != nil {
			s.TierMaxGramsWei = mustParseBigInt(*maxStr)
		}
		s.MinFeeWei = mustParseBigInt(feeStr)
		out = append(out, s)
	}
	return out, rows.Err()
}

// ── Ledger repo ────────────────────────────────────────────────────────────

type pgLedgerRepo struct{ pool *pgxpool.Pool }

func NewPGLedgerRepo(pool *pgxpool.Pool) LedgerRepo { return &pgLedgerRepo{pool: pool} }

func (r *pgLedgerRepo) Create(ctx context.Context, e domain.LedgerEntry) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO fee.ledger_entries
			(id, order_id, operation_type, amount_wei, fee_wei, fee_bps, arena, status, collected_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		e.ID, e.OrderID, e.OperationType, e.AmountWei.String(),
		e.FeeWei.String(), e.FeeBPS, e.Arena,
		e.Status, e.CollectedAt, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert ledger entry: %w", err)
	}
	return nil
}

func (r *pgLedgerRepo) List(ctx context.Context, limit, offset int) ([]domain.LedgerEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, order_id, operation_type, amount_wei, fee_wei, fee_bps, arena, status, collected_at, created_at
		 FROM fee.ledger_entries ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list ledger entries: %w", err)
	}
	defer rows.Close()

	var out []domain.LedgerEntry
	for rows.Next() {
		var (
			e      domain.LedgerEntry
			amtStr string
			feeStr string
		)
		if err := rows.Scan(
			&e.ID, &e.OrderID, &e.OperationType, &amtStr,
			&feeStr, &e.FeeBPS, &e.Arena,
			&e.Status, &e.CollectedAt, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		e.AmountWei = mustParseBigInt(amtStr)
		e.FeeWei = mustParseBigInt(feeStr)
		out = append(out, e)
	}
	return out, rows.Err()
}

func mustParseBigInt(s string) *big.Int {
	n := new(big.Int)
	n.SetString(s, 10)
	return n
}

var _ = time.Now
