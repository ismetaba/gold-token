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

// MonitoringRepo persists monitoring schedule entries.
type MonitoringRepo interface {
	// UpsertSchedule creates or updates a user's monitoring schedule.
	UpsertSchedule(ctx context.Context, s domain.MonitoringSchedule) error

	// UsersDue returns monitoring schedule entries where next_check_at <= now,
	// limited to at most `limit` results.
	UsersDue(ctx context.Context, limit int) ([]domain.MonitoringSchedule, error)

	// ScheduleByUserID returns the monitoring schedule for a single user.
	ScheduleByUserID(ctx context.Context, userID uuid.UUID) (domain.MonitoringSchedule, error)
}

type pgMonitoringRepo struct{ pool *pgxpool.Pool }

func NewPGMonitoringRepo(pool *pgxpool.Pool) MonitoringRepo { return &pgMonitoringRepo{pool: pool} }

func (r *pgMonitoringRepo) UpsertSchedule(ctx context.Context, s domain.MonitoringSchedule) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO compliance.monitoring_schedule
		   (id, user_id, last_checked_at, next_check_at, frequency_days)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id) DO UPDATE
		   SET last_checked_at = EXCLUDED.last_checked_at,
		       next_check_at   = EXCLUDED.next_check_at,
		       frequency_days  = EXCLUDED.frequency_days`,
		s.ID, s.UserID, s.LastCheckedAt, s.NextCheckAt, s.FrequencyDays,
	)
	if err != nil {
		return fmt.Errorf("upsert monitoring_schedule: %w", err)
	}
	return nil
}

func (r *pgMonitoringRepo) UsersDue(ctx context.Context, limit int) ([]domain.MonitoringSchedule, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, last_checked_at, next_check_at, frequency_days
		   FROM compliance.monitoring_schedule
		  WHERE next_check_at <= now()
		  ORDER BY next_check_at ASC
		  LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query monitoring_schedule: %w", err)
	}
	defer rows.Close()

	var out []domain.MonitoringSchedule
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *pgMonitoringRepo) ScheduleByUserID(ctx context.Context, userID uuid.UUID) (domain.MonitoringSchedule, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, last_checked_at, next_check_at, frequency_days
		   FROM compliance.monitoring_schedule WHERE user_id = $1`,
		userID,
	)
	s, err := scanSchedule(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

type scheduleScanner interface {
	Scan(dest ...any) error
}

func scanSchedule(row scheduleScanner) (domain.MonitoringSchedule, error) {
	var s domain.MonitoringSchedule
	var lastCheckedAt *time.Time
	var nextCheckAt time.Time
	err := row.Scan(&s.ID, &s.UserID, &lastCheckedAt, &nextCheckAt, &s.FrequencyDays)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, pgx.ErrNoRows
	}
	if err != nil {
		return s, fmt.Errorf("scan monitoring_schedule: %w", err)
	}
	s.LastCheckedAt = lastCheckedAt
	s.NextCheckAt = nextCheckAt
	return s, nil
}
