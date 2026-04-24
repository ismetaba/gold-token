package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/por/internal/domain"
)

// AutoAttestationConfigRepo reads and updates the singleton auto-attestation config.
type AutoAttestationConfigRepo interface {
	// Get returns the current auto-attestation configuration.
	// Returns ErrNotFound if no row exists.
	Get(ctx context.Context) (domain.AutoAttestationConfig, error)

	// UpdateLastRunAt sets last_run_at to now for the given config row.
	UpdateLastRunAt(ctx context.Context, cfg domain.AutoAttestationConfig) error
}

type pgAutoAttestationConfigRepo struct{ pool *pgxpool.Pool }

// NewPGAutoAttestationConfigRepo returns a PostgreSQL-backed AutoAttestationConfigRepo.
func NewPGAutoAttestationConfigRepo(pool *pgxpool.Pool) AutoAttestationConfigRepo {
	return &pgAutoAttestationConfigRepo{pool: pool}
}

func (r *pgAutoAttestationConfigRepo) Get(ctx context.Context) (domain.AutoAttestationConfig, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, cron_expression, enabled, last_run_at
		FROM por.auto_attestation_config
		LIMIT 1`)

	var cfg domain.AutoAttestationConfig
	var lastRunAt *time.Time
	if err := row.Scan(&cfg.ID, &cfg.CronExpression, &cfg.Enabled, &lastRunAt); err != nil {
		if err == pgx.ErrNoRows {
			return domain.AutoAttestationConfig{}, ErrNotFound
		}
		return domain.AutoAttestationConfig{}, fmt.Errorf("get auto attestation config: %w", err)
	}
	cfg.LastRunAt = lastRunAt
	return cfg, nil
}

func (r *pgAutoAttestationConfigRepo) UpdateLastRunAt(ctx context.Context, cfg domain.AutoAttestationConfig) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE por.auto_attestation_config
		SET last_run_at = $1
		WHERE id = $2`,
		now, cfg.ID,
	)
	if err != nil {
		return fmt.Errorf("update auto attestation config last_run_at: %w", err)
	}
	return nil
}
