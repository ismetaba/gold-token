package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/compliance/internal/domain"
)

// PEPRepo persists PEP check results.
type PEPRepo interface {
	SavePEPCheck(ctx context.Context, c domain.PEPCheck) error
	LatestPEPCheck(ctx context.Context, userID uuid.UUID) (domain.PEPCheck, error)
}

type pgPEPRepo struct{ pool *pgxpool.Pool }

func NewPGPEPRepo(pool *pgxpool.Pool) PEPRepo { return &pgPEPRepo{pool: pool} }

func (r *pgPEPRepo) SavePEPCheck(ctx context.Context, c domain.PEPCheck) error {
	detailsJSON, err := json.Marshal(c.MatchDetails)
	if err != nil {
		return fmt.Errorf("marshal pep match_details: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO compliance.pep_checks (id, user_id, matched, match_details, checked_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		c.ID, c.UserID, c.Matched, detailsJSON, c.CheckedAt,
	)
	if err != nil {
		return fmt.Errorf("insert pep_check: %w", err)
	}
	return nil
}

func (r *pgPEPRepo) LatestPEPCheck(ctx context.Context, userID uuid.UUID) (domain.PEPCheck, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, matched, match_details, checked_at
		   FROM compliance.pep_checks
		  WHERE user_id = $1
		  ORDER BY checked_at DESC
		  LIMIT 1`,
		userID,
	)
	var c domain.PEPCheck
	var detailsJSON []byte
	var checkedAt time.Time
	if err := row.Scan(&c.ID, &c.UserID, &c.Matched, &detailsJSON, &checkedAt); err != nil {
		return c, fmt.Errorf("scan pep_check: %w", err)
	}
	c.CheckedAt = checkedAt
	if len(detailsJSON) > 0 {
		if err := json.Unmarshal(detailsJSON, &c.MatchDetails); err != nil {
			return c, fmt.Errorf("unmarshal pep match_details: %w", err)
		}
	}
	return c, nil
}
