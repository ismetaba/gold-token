package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/compliance/internal/domain"
	"github.com/ismetaba/gold-token/backend/services/compliance/internal/jurisdiction"
)

type pgJurisdictionRepo struct{ pool *pgxpool.Pool }

func NewPGJurisdictionRepo(pool *pgxpool.Pool) jurisdiction.RuleRepo {
	return &pgJurisdictionRepo{pool: pool}
}

func (r *pgJurisdictionRepo) ListActiveRules(ctx context.Context, arena string) ([]domain.JurisdictionRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, arena, rule_type, threshold_grams_wei::TEXT, action, active
		   FROM compliance.jurisdiction_rules
		  WHERE active = TRUE AND arena = $1
		  ORDER BY rule_type`,
		arena,
	)
	if err != nil {
		return nil, fmt.Errorf("list active jurisdiction rules: %w", err)
	}
	defer rows.Close()
	return scanRules(rows)
}

func (r *pgJurisdictionRepo) ListAllRules(ctx context.Context) ([]domain.JurisdictionRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, arena, rule_type, threshold_grams_wei::TEXT, action, active
		   FROM compliance.jurisdiction_rules
		  ORDER BY arena, rule_type`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all jurisdiction rules: %w", err)
	}
	defer rows.Close()
	return scanRules(rows)
}

func (r *pgJurisdictionRepo) GetRule(ctx context.Context, id string) (domain.JurisdictionRule, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, arena, rule_type, threshold_grams_wei::TEXT, action, active
		   FROM compliance.jurisdiction_rules WHERE id = $1`,
		id,
	)
	rule, err := scanRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rule, ErrNotFound
	}
	return rule, err
}

func (r *pgJurisdictionRepo) UpdateRule(ctx context.Context, id string, active bool, action string) (domain.JurisdictionRule, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE compliance.jurisdiction_rules
		    SET active = $2, action = $3, updated_at = now()
		  WHERE id = $1
		  RETURNING id, arena, rule_type, threshold_grams_wei::TEXT, action, active`,
		id, active, action,
	)
	rule, err := scanRule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return rule, ErrNotFound
	}
	return rule, err
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

type ruleScanner interface {
	Scan(dest ...any) error
}

func scanRule(row ruleScanner) (domain.JurisdictionRule, error) {
	var rule domain.JurisdictionRule
	var threshold *string
	err := row.Scan(&rule.ID, &rule.Arena, &rule.RuleType, &threshold, &rule.Action, &rule.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return rule, pgx.ErrNoRows
	}
	if err != nil {
		return rule, fmt.Errorf("scan jurisdiction_rule: %w", err)
	}
	rule.ThresholdGramsWei = threshold
	return rule, nil
}

func scanRules(rows interface {
	Next() bool
	Err() error
	Scan(dest ...any) error
	Close()
}) ([]domain.JurisdictionRule, error) {
	defer rows.Close()
	var out []domain.JurisdictionRule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}
