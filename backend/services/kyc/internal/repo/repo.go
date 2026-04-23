// Package repo provides PostgreSQL-backed KYC persistence.
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/kyc/internal/domain"
)

var (
	ErrNotFound       = errors.New("application not found")
	ErrAlreadyActive  = errors.New("user already has an active KYC application")
)

// ApplicationRepo is the interface for KYC application persistence.
type ApplicationRepo interface {
	Create(ctx context.Context, app domain.Application) error
	ByID(ctx context.Context, id uuid.UUID) (domain.Application, error)
	ByUserID(ctx context.Context, userID uuid.UUID) (domain.Application, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, reviewerNote string, reviewedAt *time.Time) error
}

type pgRepo struct {
	pool *pgxpool.Pool
}

// NewPGRepo returns a PostgreSQL-backed ApplicationRepo.
func NewPGRepo(pool *pgxpool.Pool) ApplicationRepo {
	return &pgRepo{pool: pool}
}

func (r *pgRepo) Create(ctx context.Context, app domain.Application) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO kyc.applications
			(id, user_id, status, document_path, first_name, last_name, date_of_birth, nationality, reviewer_note, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		app.ID, app.UserID, app.Status, app.DocumentPath,
		app.FirstName, app.LastName, app.DateOfBirth, app.Nationality,
		app.ReviewerNote, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrAlreadyActive
		}
		return fmt.Errorf("insert application: %w", err)
	}
	return nil
}

func (r *pgRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Application, error) {
	return r.scanOne(ctx,
		`SELECT id, user_id, status, document_path, first_name, last_name, date_of_birth, nationality,
		        reviewer_note, created_at, updated_at, reviewed_at
		 FROM kyc.applications WHERE id = $1`, id)
}

func (r *pgRepo) ByUserID(ctx context.Context, userID uuid.UUID) (domain.Application, error) {
	return r.scanOne(ctx,
		`SELECT id, user_id, status, document_path, first_name, last_name, date_of_birth, nationality,
		        reviewer_note, created_at, updated_at, reviewed_at
		 FROM kyc.applications WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT 1`, userID)
}

func (r *pgRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, reviewerNote string, reviewedAt *time.Time) error {
	now := time.Now().UTC()
	tag, err := r.pool.Exec(ctx,
		`UPDATE kyc.applications
		 SET status = $2, reviewer_note = $3, reviewed_at = $4, updated_at = $5
		 WHERE id = $1`,
		id, status, reviewerNote, reviewedAt, now,
	)
	if err != nil {
		return fmt.Errorf("update application status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *pgRepo) scanOne(ctx context.Context, query string, args ...any) (domain.Application, error) {
	var a domain.Application
	var dob time.Time
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&a.ID, &a.UserID, &a.Status, &a.DocumentPath,
		&a.FirstName, &a.LastName, &dob, &a.Nationality,
		&a.ReviewerNote, &a.CreatedAt, &a.UpdatedAt, &a.ReviewedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, fmt.Errorf("scan application: %w", err)
	}
	a.DateOfBirth = dob.Format("2006-01-02")
	return a, nil
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
