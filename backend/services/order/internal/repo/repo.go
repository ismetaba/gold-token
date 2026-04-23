// Package repo provides PostgreSQL-backed persistence for the order service.
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/order/internal/domain"
)

var ErrNotFound = errors.New("order: not found")
var ErrAlreadyExists = errors.New("order: idempotency key already exists")

// OrderRepo persists orders.
type OrderRepo interface {
	Create(ctx context.Context, o domain.Order) error
	ByID(ctx context.Context, id uuid.UUID) (domain.Order, error)
	ByIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (domain.Order, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Order, error)
	Update(ctx context.Context, o domain.Order) error
}

type pgOrderRepo struct{ pool *pgxpool.Pool }

func NewPGOrderRepo(pool *pgxpool.Pool) OrderRepo { return &pgOrderRepo{pool: pool} }

const cols = `id, user_id, type, status, amount_grams, amount_wei, user_address, arena,
              allocation_id, idempotency_key, failure_reason,
              created_at, updated_at, confirmed_at, completed_at`

func (r *pgOrderRepo) Create(ctx context.Context, o domain.Order) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO orders.orders
		   (id, user_id, type, status, amount_grams, amount_wei, user_address, arena,
		    allocation_id, idempotency_key, failure_reason,
		    created_at, updated_at, confirmed_at, completed_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		o.ID, o.UserID, o.Type, o.Status,
		o.AmountGrams, o.AmountWei, o.UserAddress, o.Arena,
		o.AllocationID, o.IdempotencyKey, o.FailureReason,
		o.CreatedAt, o.UpdatedAt, o.ConfirmedAt, o.CompletedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("insert order: %w", err)
	}
	return nil
}

func (r *pgOrderRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Order, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+cols+` FROM orders.orders WHERE id = $1`, id,
	)
	return scanOrder(row)
}

func (r *pgOrderRepo) ByIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (domain.Order, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+cols+` FROM orders.orders WHERE user_id = $1 AND idempotency_key = $2`,
		userID, key,
	)
	return scanOrder(row)
}

func (r *pgOrderRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+cols+` FROM orders.orders
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	var out []domain.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *pgOrderRepo) Update(ctx context.Context, o domain.Order) error {
	o.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx,
		`UPDATE orders.orders SET
		   status          = $2,
		   allocation_id   = $3,
		   failure_reason  = $4,
		   updated_at      = $5,
		   confirmed_at    = $6,
		   completed_at    = $7
		 WHERE id = $1`,
		o.ID, o.Status, o.AllocationID, o.FailureReason,
		o.UpdatedAt, o.ConfirmedAt, o.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(row scanner) (domain.Order, error) {
	var o domain.Order
	var confirmedAt, completedAt *time.Time
	err := row.Scan(
		&o.ID, &o.UserID, &o.Type, &o.Status,
		&o.AmountGrams, &o.AmountWei, &o.UserAddress, &o.Arena,
		&o.AllocationID, &o.IdempotencyKey, &o.FailureReason,
		&o.CreatedAt, &o.UpdatedAt, &confirmedAt, &completedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	if err != nil {
		return o, fmt.Errorf("scan order: %w", err)
	}
	o.ConfirmedAt = confirmedAt
	o.CompletedAt = completedAt
	return o, nil
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
