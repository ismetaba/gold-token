// Package repo provides PostgreSQL-backed Audit Log persistence.
package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/audit/internal/domain"
)

var ErrNotFound = errors.New("record not found")

// EntryRepo persists audit entries (append-only).
type EntryRepo interface {
	Insert(ctx context.Context, e domain.Entry) error
	ByID(ctx context.Context, id uuid.UUID) (domain.Entry, error)
	List(ctx context.Context, f domain.ListFilter) ([]domain.Entry, error)
}

type pgEntryRepo struct{ pool *pgxpool.Pool }

// NewPGEntryRepo returns a PostgreSQL-backed EntryRepo.
func NewPGEntryRepo(pool *pgxpool.Pool) EntryRepo { return &pgEntryRepo{pool: pool} }

func (r *pgEntryRepo) Insert(ctx context.Context, e domain.Entry) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit.entries
			(id, event_id, event_type, actor_id, actor_type, entity_id, entity_type, action, metadata, occurred_at, ingested_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (event_id, occurred_at) DO NOTHING`,
		e.ID, e.EventID, e.EventType, e.ActorID, e.ActorType,
		e.EntityID, e.EntityType, e.Action, e.Metadata,
		e.OccurredAt, e.IngestedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}
	return nil
}

func (r *pgEntryRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Entry, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, event_id, event_type, actor_id, actor_type, entity_id, entity_type, action, metadata, occurred_at, ingested_at
		 FROM audit.entries WHERE id = $1`, id)

	var e domain.Entry
	err := row.Scan(
		&e.ID, &e.EventID, &e.EventType, &e.ActorID, &e.ActorType,
		&e.EntityID, &e.EntityType, &e.Action, &e.Metadata,
		&e.OccurredAt, &e.IngestedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Entry{}, ErrNotFound
		}
		return domain.Entry{}, fmt.Errorf("query audit entry: %w", err)
	}
	return e, nil
}

func (r *pgEntryRepo) List(ctx context.Context, f domain.ListFilter) ([]domain.Entry, error) {
	var (
		where []string
		args  []any
		idx   = 1
	)

	if f.EntityType != nil {
		where = append(where, fmt.Sprintf("entity_type = $%d", idx))
		args = append(args, *f.EntityType)
		idx++
	}
	if f.EntityID != nil {
		where = append(where, fmt.Sprintf("entity_id = $%d", idx))
		args = append(args, *f.EntityID)
		idx++
	}
	if f.ActorID != nil {
		where = append(where, fmt.Sprintf("actor_id = $%d", idx))
		args = append(args, *f.ActorID)
		idx++
	}
	if f.Action != nil {
		where = append(where, fmt.Sprintf("action = $%d", idx))
		args = append(args, *f.Action)
		idx++
	}
	if f.From != nil {
		where = append(where, fmt.Sprintf("occurred_at >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where = append(where, fmt.Sprintf("occurred_at <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}

	q := `SELECT id, event_id, event_type, actor_id, actor_type, entity_id, entity_type, action, metadata, occurred_at, ingested_at
	      FROM audit.entries`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY occurred_at DESC"
	q += fmt.Sprintf(" LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

func scanEntries(rows pgx.Rows) ([]domain.Entry, error) {
	var out []domain.Entry
	for rows.Next() {
		var e domain.Entry
		if err := rows.Scan(
			&e.ID, &e.EventID, &e.EventType, &e.ActorID, &e.ActorType,
			&e.EntityID, &e.EntityType, &e.Action, &e.Metadata,
			&e.OccurredAt, &e.IngestedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Ensure time.Time is imported for the List filter.
var _ = time.Now
