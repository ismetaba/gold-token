// Package repo provides PostgreSQL persistence for the market data service.
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/price-oracle/internal/domain"
)

// PriceRepo persists and retrieves price snapshots and candles.
type PriceRepo interface {
	Save(ctx context.Context, p domain.Price) error
	History(ctx context.Context, pair string, since time.Time) ([]domain.Price, error)
	UpsertCandle(ctx context.Context, p domain.Price, interval string) error
	GetCandles(ctx context.Context, pair, interval string, from, to time.Time) ([]domain.Candle, error)
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
		`INSERT INTO oracle.price_history (id, pair, price_per_gram, provider, fetched_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		p.ID, p.Pair, p.PricePerGram, p.Provider, p.FetchedAt,
	)
	if err != nil {
		return fmt.Errorf("save price: %w", err)
	}
	return nil
}

func (r *pgPriceRepo) History(ctx context.Context, pair string, since time.Time) ([]domain.Price, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, pair, price_per_gram, provider, fetched_at
		 FROM oracle.price_history
		 WHERE pair = $1 AND fetched_at >= $2
		 ORDER BY fetched_at ASC`,
		pair, since,
	)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var prices []domain.Price
	for rows.Next() {
		var p domain.Price
		if err := rows.Scan(&p.ID, &p.Pair, &p.PricePerGram, &p.Provider, &p.FetchedAt); err != nil {
			return nil, fmt.Errorf("scan price row: %w", err)
		}
		prices = append(prices, p)
	}
	return prices, rows.Err()
}

// UpsertCandle inserts or updates the OHLCV candle for the bucket that
// contains p.FetchedAt for the given interval ("1h", "4h", "1d").
// The candle is upserted so repeated fetches within the same bucket
// update high/low/close without duplicating rows.
func (r *pgPriceRepo) UpsertCandle(ctx context.Context, p domain.Price, interval string) error {
	bucketStart, bucketEnd, err := bucketBounds(p.FetchedAt, interval)
	if err != nil {
		return fmt.Errorf("upsert candle: %w", err)
	}

	_, dbErr := r.pool.Exec(ctx, `
		INSERT INTO oracle.candles
		    (pair, interval, open_per_gram, high_per_gram, low_per_gram, close_per_gram, volume, bucket_start, bucket_end)
		VALUES
		    ($1, $2, $3, $3, $3, $3, 0, $4, $5)
		ON CONFLICT (pair, interval, bucket_start) DO UPDATE SET
		    high_per_gram  = GREATEST(oracle.candles.high_per_gram,  EXCLUDED.high_per_gram),
		    low_per_gram   = LEAST   (oracle.candles.low_per_gram,   EXCLUDED.low_per_gram),
		    close_per_gram = EXCLUDED.close_per_gram
	`, p.Pair, interval, p.PricePerGram, bucketStart, bucketEnd)

	if dbErr != nil {
		return fmt.Errorf("upsert candle: %w", dbErr)
	}
	return nil
}

func (r *pgPriceRepo) GetCandles(
	ctx context.Context,
	pair, interval string,
	from, to time.Time,
) ([]domain.Candle, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, pair, interval, open_per_gram, high_per_gram, low_per_gram, close_per_gram, volume, bucket_start, bucket_end
		FROM oracle.candles
		WHERE pair = $1 AND interval = $2
		  AND bucket_start >= $3 AND bucket_start < $4
		ORDER BY bucket_start ASC
	`, pair, interval, from, to)
	if err != nil {
		return nil, fmt.Errorf("get candles: %w", err)
	}
	defer rows.Close()

	var candles []domain.Candle
	for rows.Next() {
		var c domain.Candle
		if err := rows.Scan(
			&c.ID, &c.Pair, &c.Interval,
			&c.OpenPerGram, &c.HighPerGram, &c.LowPerGram, &c.ClosePerGram,
			&c.Volume, &c.BucketStart, &c.BucketEnd,
		); err != nil {
			return nil, fmt.Errorf("scan candle row: %w", err)
		}
		candles = append(candles, c)
	}
	return candles, rows.Err()
}

// bucketBounds computes the [start, end) time range for the candle bucket
// containing t for the given interval.
func bucketBounds(t time.Time, interval string) (start, end time.Time, err error) {
	t = t.UTC()
	switch interval {
	case "1h":
		start = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
		end = start.Add(time.Hour)
	case "4h":
		// Align to 4-hour buckets starting at midnight.
		hour := (t.Hour() / 4) * 4
		start = time.Date(t.Year(), t.Month(), t.Day(), hour, 0, 0, 0, time.UTC)
		end = start.Add(4 * time.Hour)
	case "1d":
		start = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		end = start.Add(24 * time.Hour)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported interval %q", interval)
	}
	return start, end, nil
}
