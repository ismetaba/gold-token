// Package repo provides the data access layer for the mint-burn service.
// Backed by PostgreSQL via pgx. Queries are handwritten in this skeleton;
// target is sqlc-generated in Faz 1.
package repo

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/ismetaba/gold-token/backend/services/mint-burn/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNoPending döner: polling sırasında işlenecek saga yok.
var ErrNoPending = errors.New("no pending saga")

// SagaRepo saga_instances tablosuna erişim.
type SagaRepo interface {
	Create(ctx context.Context, s *domain.Saga) error
	ByID(ctx context.Context, id uuid.UUID) (*domain.Saga, error)
	NextPending(ctx context.Context) (*domain.Saga, error)
	UpdateState(ctx context.Context, s *domain.Saga) error
}

// BarRepo gold_bars ve bar_allocations erişimi.
type BarRepo interface {
	ReserveBars(ctx context.Context, arena domain.Arena, amountWei *big.Int, sagaID, allocID uuid.UUID) ([]string, error)
	ReleaseAllocation(ctx context.Context, allocID uuid.UUID) error
	ListAllocations(ctx context.Context, allocID uuid.UUID) ([]domain.BarAllocation, error)
}

// PG implementation (skeleton — tam query'ler Faz 1'de sqlc ile).
type pgSagaRepo struct {
	pool *pgxpool.Pool
}

func NewPGSagaRepo(pool *pgxpool.Pool) SagaRepo { return &pgSagaRepo{pool: pool} }

func (r *pgSagaRepo) Create(ctx context.Context, s *domain.Saga) error {
	const q = `
		INSERT INTO mint.saga_instances
		  (id, saga_type, state, order_id, arena, context, started_at, last_step_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
	`
	_, err := r.pool.Exec(ctx, q,
		s.ID, s.Type, s.State, s.OrderID, s.Arena, &s.Context, s.StartedAt,
	)
	return err
}

func (r *pgSagaRepo) ByID(ctx context.Context, id uuid.UUID) (*domain.Saga, error) {
	const q = `
		SELECT id, saga_type, state, order_id, arena, context,
		       started_at, last_step_at, completed_at, attempts
		FROM mint.saga_instances
		WHERE id = $1
	`
	var s domain.Saga
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&s.ID, &s.Type, &s.State, &s.OrderID, &s.Arena, &s.Context,
		&s.StartedAt, &s.LastStepAt, &s.CompletedAt, &s.Attempts,
	)
	return &s, err
}

// NextPending, en eski terminal-olmayan saga'yı SKIP LOCKED ile kilitler.
// Advisory lock ile birden fazla worker aynı saga'ya dokunmaz.
func (r *pgSagaRepo) NextPending(ctx context.Context) (*domain.Saga, error) {
	const q = `
		SELECT id, saga_type, state, order_id, arena, context,
		       started_at, last_step_at, completed_at, attempts
		FROM mint.saga_instances
		WHERE completed_at IS NULL
		  AND state NOT IN ('completed','failed','failed_no_stock',
		                    'failed_approval_timeout','failed_reserve_invariant',
		                    'burn_executed')
		  AND last_step_at < now() - interval '5 seconds'
		ORDER BY last_step_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`
	var s domain.Saga
	err := r.pool.QueryRow(ctx, q).Scan(
		&s.ID, &s.Type, &s.State, &s.OrderID, &s.Arena, &s.Context,
		&s.StartedAt, &s.LastStepAt, &s.CompletedAt, &s.Attempts,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *pgSagaRepo) UpdateState(ctx context.Context, s *domain.Saga) error {
	const q = `
		UPDATE mint.saga_instances
		SET state = $2,
		    context = $3,
		    last_step_at = $4,
		    completed_at = $5,
		    attempts = attempts + 1
		WHERE id = $1
	`
	var completedAt *time.Time
	if s.State.IsTerminal() {
		now := time.Now().UTC()
		completedAt = &now
	}
	_, err := r.pool.Exec(ctx, q, s.ID, s.State, &s.Context, time.Now().UTC(), completedAt)
	return err
}

// BarRepo skeleton — gerçek implementasyon Faz 1'de sqlc ile.
type pgBarRepo struct {
	pool *pgxpool.Pool
}

func NewPGBarRepo(pool *pgxpool.Pool) BarRepo { return &pgBarRepo{pool: pool} }

// ReserveBars: SELECT FOR UPDATE SKIP LOCKED, FIFO allocation.
// Returns bar seri listesi. Yetersiz kasa ise ErrNoPending tarzı bir ek hata döner.
func (r *pgBarRepo) ReserveBars(
	ctx context.Context,
	arena domain.Arena,
	amountWei *big.Int,
	sagaID, allocID uuid.UUID,
) ([]string, error) {
	// TODO(Faz1): tam implementation. Şu an placeholder.
	// 1. BEGIN tx
	// 2. SELECT bars WHERE vault matches arena + free weight ≥ remaining, SKIP LOCKED, ORDER BY cast_date
	// 3. Loop: allocate needed amount from each bar, insert bar_allocations, update gold_bars.allocated_sum
	// 4. COMMIT
	return nil, errors.New("repo.ReserveBars: not implemented (faz 1)")
}

func (r *pgBarRepo) ReleaseAllocation(ctx context.Context, allocID uuid.UUID) error {
	// TODO(Faz1): UPDATE bar_allocations SET released_at=now() WHERE allocation_id=$1;
	//             UPDATE gold_bars SET allocated_sum = allocated_sum - released_wei ...
	return errors.New("repo.ReleaseAllocation: not implemented (faz 1)")
}

func (r *pgBarRepo) ListAllocations(ctx context.Context, allocID uuid.UUID) ([]domain.BarAllocation, error) {
	return nil, errors.New("repo.ListAllocations: not implemented (faz 1)")
}
