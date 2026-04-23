// Package repo provides PostgreSQL persistence for price oracle history.
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/domain"
)

// PriceRepo persists and retrieves price snapshots.
type PriceRepo interface {
	Save(ctx context.Context, p domain.Price) error
	History(ctx context.Context, since time.Time) ([]domain.Price, error)
}

type pgPriceRepo struct {
	pool *pgxpool.Pool
}

// NewPGPriceRepo returns a PostgreSQL-backed PriceRepo.
func NewPGPriceRepo(pool *pgxpool.Pool) PriceRepo {
	return &pgPriceRepo{pool: pool}
}

func (r *pgPriceRepo) Save(ctx context.Context, p domain.Price) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO oracle.price_history (id, price_usd_g, provider, fetched_at)
		 VALUES ($1, $2, $3, $4)`,
		p.ID, p.PriceUSDg, p.Provider, p.FetchedAt,
	)
	if err != nil {
		return fmt.Errorf("save price: %w", err)
	}
	return nil
}

func (r *pgPriceRepo) History(ctx context.Context, since time.Time) ([]domain.Price, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, price_usd_g, provider, fetched_at
		 FROM oracle.price_history
		 WHERE fetched_at >= $1
		 ORDER BY fetched_at ASC`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var prices []domain.Price
	for rows.Next() {
		var p domain.Price
		if err := rows.Scan(&p.ID, &p.PriceUSDg, &p.Provider, &p.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan price row: %w", err)
		}
		prices = append(prices, p)
	}
	return prices, rows.Err()
}
