// Package repo provides PostgreSQL-backed persistence for the compliance service.
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/compliance/internal/domain"
)

var ErrNotFound = errors.New("compliance: not found")

// ComplianceRepo persists screening results and user compliance state.
type ComplianceRepo interface {
	// SaveResult stores a new screening result.
	SaveResult(ctx context.Context, r domain.ScreeningResult) error

	// ResultsByUserID returns all results for a user, newest first.
	ResultsByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]domain.ScreeningResult, error)

	// UpsertState sets the aggregate compliance status for a user.
	UpsertState(ctx context.Context, s domain.ComplianceState) error

	// StateByUserID returns the current compliance state for a user.
	StateByUserID(ctx context.Context, userID uuid.UUID) (domain.ComplianceState, error)
}

type pgRepo struct{ pool *pgxpool.Pool }

func NewPGRepo(pool *pgxpool.Pool) ComplianceRepo { return &pgRepo{pool: pool} }

// ─────────────────────────────────────────────────────────────────────────────
// ScreeningResult
// ─────────────────────────────────────────────────────────────────────────────

func (r *pgRepo) SaveResult(ctx context.Context, res domain.ScreeningResult) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO compliance.screening_results
		   (id, user_id, order_id, status, match_type, matched_name, provider, screened_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		res.ID, res.UserID, res.OrderID,
		res.Status, res.MatchType, res.MatchedName,
		res.Provider, res.ScreenedAt,
	)
	if err != nil {
		return fmt.Errorf("insert screening_result: %w", err)
	}
	return nil
}

func (r *pgRepo) ResultsByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]domain.ScreeningResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, order_id, status, match_type, matched_name, provider, screened_at
		   FROM compliance.screening_results
		  WHERE user_id = $1
		  ORDER BY screened_at DESC
		  LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query screening_results: %w", err)
	}
	defer rows.Close()

	var out []domain.ScreeningResult
	for rows.Next() {
		res, err := scanResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

// ─────────────────────────────────────────────────────────────────────────────
// ComplianceState
// ─────────────────────────────────────────────────────────────────────────────

func (r *pgRepo) UpsertState(ctx context.Context, s domain.ComplianceState) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO compliance.user_status (user_id, status, updated_at)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (user_id) DO UPDATE
		   SET status     = EXCLUDED.status,
		       updated_at = EXCLUDED.updated_at`,
		s.UserID, s.Status, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert user_status: %w", err)
	}
	return nil
}

func (r *pgRepo) StateByUserID(ctx context.Context, userID uuid.UUID) (domain.ComplianceState, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT user_id, status, updated_at FROM compliance.user_status WHERE user_id = $1`,
		userID,
	)
	var s domain.ComplianceState
	err := row.Scan(&s.UserID, &s.Status, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	if err != nil {
		return s, fmt.Errorf("scan user_status: %w", err)
	}
	return s, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

type scanner interface {
	Scan(dest ...any) error
}

func scanResult(row scanner) (domain.ScreeningResult, error) {
	var res domain.ScreeningResult
	var screenedAt time.Time
	err := row.Scan(
		&res.ID, &res.UserID, &res.OrderID,
		&res.Status, &res.MatchType, &res.MatchedName,
		&res.Provider, &screenedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrNotFound
	}
	if err != nil {
		return res, fmt.Errorf("scan screening_result: %w", err)
	}
	res.ScreenedAt = screenedAt
	return res, nil
}
