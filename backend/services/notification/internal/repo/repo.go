package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/notification/internal/domain"
)

var ErrNotFound = errors.New("record not found")

// UserEmailRepo resolves a user's email from the auth schema.
// The notification service shares the same Postgres instance as auth.
type UserEmailRepo interface {
	EmailByUserID(ctx context.Context, userID uuid.UUID) (string, error)
}

type pgUserEmailRepo struct{ pool *pgxpool.Pool }

func NewPGUserEmailRepo(pool *pgxpool.Pool) UserEmailRepo { return &pgUserEmailRepo{pool: pool} }

func (r *pgUserEmailRepo) EmailByUserID(ctx context.Context, userID uuid.UUID) (string, error) {
	var email string
	err := r.pool.QueryRow(ctx, `SELECT email FROM auth.users WHERE id = $1`, userID).Scan(&email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("lookup user email: %w", err)
	}
	return email, nil
}

type DeliveryRepo interface {
	Create(ctx context.Context, d domain.Delivery) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Delivery, error)
	MarkRead(ctx context.Context, id, userID uuid.UUID) error
}

type TemplateRepo interface {
	ByEventType(ctx context.Context, eventType string) (domain.Template, error)
}

type PreferencesRepo interface {
	ByUserID(ctx context.Context, userID uuid.UUID) (domain.Preferences, error)
	Upsert(ctx context.Context, p domain.Preferences) error
}

// ── Delivery ───────────────────────────────────────────────────────────────

type pgDeliveryRepo struct{ pool *pgxpool.Pool }

func NewPGDeliveryRepo(pool *pgxpool.Pool) DeliveryRepo { return &pgDeliveryRepo{pool: pool} }

func (r *pgDeliveryRepo) Create(ctx context.Context, d domain.Delivery) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notification.deliveries
			(id, user_id, template_id, channel, subject, body, status, error, sent_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		d.ID, d.UserID, d.TemplateID, d.Channel, d.Subject, d.Body,
		d.Status, d.Error, d.SentAt, d.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert delivery: %w", err)
	}
	return nil
}

func (r *pgDeliveryRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Delivery, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, template_id, channel, subject, body, status, error, sent_at, created_at
		 FROM notification.deliveries
		 WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	defer rows.Close()
	return scanDeliveries(rows)
}

func (r *pgDeliveryRepo) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE notification.deliveries SET status = 'read' WHERE id = $1 AND user_id = $2 AND status != 'read'`,
		id, userID)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanDeliveries(rows pgx.Rows) ([]domain.Delivery, error) {
	var out []domain.Delivery
	for rows.Next() {
		var d domain.Delivery
		if err := rows.Scan(
			&d.ID, &d.UserID, &d.TemplateID, &d.Channel,
			&d.Subject, &d.Body, &d.Status, &d.Error,
			&d.SentAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ── Template ───────────────────────────────────────────────────────────────

type pgTemplateRepo struct{ pool *pgxpool.Pool }

func NewPGTemplateRepo(pool *pgxpool.Pool) TemplateRepo { return &pgTemplateRepo{pool: pool} }

func (r *pgTemplateRepo) ByEventType(ctx context.Context, eventType string) (domain.Template, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, event_type, subject_template, body_template, channels, active
		 FROM notification.templates WHERE event_type = $1 AND active = true`, eventType)

	var t domain.Template
	err := row.Scan(&t.ID, &t.EventType, &t.SubjectTemplate, &t.BodyTemplate, &t.Channels, &t.Active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Template{}, ErrNotFound
		}
		return domain.Template{}, fmt.Errorf("query template: %w", err)
	}
	return t, nil
}

// ── Preferences ────────────────────────────────────────────────────────────

type pgPreferencesRepo struct{ pool *pgxpool.Pool }

func NewPGPreferencesRepo(pool *pgxpool.Pool) PreferencesRepo {
	return &pgPreferencesRepo{pool: pool}
}

func (r *pgPreferencesRepo) ByUserID(ctx context.Context, userID uuid.UUID) (domain.Preferences, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, email_enabled, webhook_url, webhook_enabled, inapp_enabled, updated_at
		 FROM notification.preferences WHERE user_id = $1`, userID)

	var p domain.Preferences
	err := row.Scan(&p.ID, &p.UserID, &p.EmailEnabled, &p.WebhookURL, &p.WebhookEnabled, &p.InappEnabled, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Return defaults.
			return domain.Preferences{
				UserID:       userID,
				EmailEnabled: true,
				InappEnabled: true,
			}, nil
		}
		return domain.Preferences{}, fmt.Errorf("query preferences: %w", err)
	}
	return p, nil
}

func (r *pgPreferencesRepo) Upsert(ctx context.Context, p domain.Preferences) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notification.preferences (id, user_id, email_enabled, webhook_url, webhook_enabled, inapp_enabled, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (user_id) DO UPDATE SET
			email_enabled = EXCLUDED.email_enabled,
			webhook_url = EXCLUDED.webhook_url,
			webhook_enabled = EXCLUDED.webhook_enabled,
			inapp_enabled = EXCLUDED.inapp_enabled,
			updated_at = EXCLUDED.updated_at`,
		p.ID, p.UserID, p.EmailEnabled, p.WebhookURL, p.WebhookEnabled, p.InappEnabled, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert preferences: %w", err)
	}
	return nil
}

var _ = time.Now
