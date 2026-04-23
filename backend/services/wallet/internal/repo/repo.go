// Package repo provides PostgreSQL-backed persistence for the wallet service.
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/wallet/internal/domain"
)

var ErrNotFound = errors.New("wallet: not found")
var ErrAlreadyExists = errors.New("wallet: already exists")

// ─────────────────────────────────────────────────────────────────────────────
// WalletRepo
// ─────────────────────────────────────────────────────────────────────────────

// WalletRepo persists user wallets (user_id → ethereum address mapping).
type WalletRepo interface {
	Create(ctx context.Context, w domain.Wallet) error
	ByUserID(ctx context.Context, userID uuid.UUID) (domain.Wallet, error)
	ByAddress(ctx context.Context, address string) (domain.Wallet, error)
}

type pgWalletRepo struct{ pool *pgxpool.Pool }

func NewPGWalletRepo(pool *pgxpool.Pool) WalletRepo { return &pgWalletRepo{pool: pool} }

func (r *pgWalletRepo) Create(ctx context.Context, w domain.Wallet) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO wallet.wallets (id, user_id, address, created_at)
		 VALUES ($1, $2, $3, $4)`,
		w.ID, w.UserID, w.Address, w.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("insert wallet: %w", err)
	}
	return nil
}

func (r *pgWalletRepo) ByUserID(ctx context.Context, userID uuid.UUID) (domain.Wallet, error) {
	var w domain.Wallet
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, address, created_at FROM wallet.wallets WHERE user_id = $1`,
		userID,
	).Scan(&w.ID, &w.UserID, &w.Address, &w.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return w, ErrNotFound
	}
	if err != nil {
		return w, fmt.Errorf("query wallet by user: %w", err)
	}
	return w, nil
}

func (r *pgWalletRepo) ByAddress(ctx context.Context, address string) (domain.Wallet, error) {
	var w domain.Wallet
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, address, created_at FROM wallet.wallets WHERE lower(address) = lower($1)`,
		address,
	).Scan(&w.ID, &w.UserID, &w.Address, &w.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return w, ErrNotFound
	}
	if err != nil {
		return w, fmt.Errorf("query wallet by address: %w", err)
	}
	return w, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// TxRepo
// ─────────────────────────────────────────────────────────────────────────────

// TxRepo persists the event-sourced transaction log.
type TxRepo interface {
	Create(ctx context.Context, tx domain.Transaction) error
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Transaction, error)
}

type pgTxRepo struct{ pool *pgxpool.Pool }

func NewPGTxRepo(pool *pgxpool.Pool) TxRepo { return &pgTxRepo{pool: pool} }

func (r *pgTxRepo) Create(ctx context.Context, tx domain.Transaction) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO wallet.transaction_log
		   (id, user_id, address, tx_hash, event_type, amount_wei, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (tx_hash, event_type) DO NOTHING`,
		tx.ID, tx.UserID, tx.Address, tx.TxHash, tx.EventType, tx.AmountWei, tx.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("insert transaction: %w", err)
	}
	return nil
}

func (r *pgTxRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Transaction, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, address, tx_hash, event_type, amount_wei, occurred_at
		 FROM wallet.transaction_log
		 WHERE user_id = $1
		 ORDER BY occurred_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var out []domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		var occurredAt time.Time
		if err := rows.Scan(&tx.ID, &tx.UserID, &tx.Address, &tx.TxHash, &tx.EventType, &tx.AmountWei, &occurredAt); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		tx.OccurredAt = occurredAt
		out = append(out, tx)
	}
	return out, rows.Err()
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
