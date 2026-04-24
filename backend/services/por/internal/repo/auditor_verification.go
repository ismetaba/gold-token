package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/por/internal/domain"
)

// AuditorVerificationRepo persists third-party auditor verification records.
type AuditorVerificationRepo interface {
	// Create inserts a new verification record.
	Create(ctx context.Context, v domain.AuditorVerification) error

	// ListByAttestation returns all verifications for the given attestation, newest first.
	ListByAttestation(ctx context.Context, attestationID uuid.UUID) ([]domain.AuditorVerification, error)
}

type pgAuditorVerificationRepo struct{ pool *pgxpool.Pool }

// NewPGAuditorVerificationRepo returns a PostgreSQL-backed AuditorVerificationRepo.
func NewPGAuditorVerificationRepo(pool *pgxpool.Pool) AuditorVerificationRepo {
	return &pgAuditorVerificationRepo{pool: pool}
}

func (r *pgAuditorVerificationRepo) Create(ctx context.Context, v domain.AuditorVerification) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO por.auditor_verifications
		  (id, attestation_id, auditor_name, auditor_id, verification_hash, verified_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		v.ID, v.AttestationID, v.AuditorName, v.AuditorID, v.VerificationHash, v.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("insert auditor verification: %w", err)
	}
	return nil
}

func (r *pgAuditorVerificationRepo) ListByAttestation(ctx context.Context, attestationID uuid.UUID) ([]domain.AuditorVerification, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, attestation_id, auditor_name, auditor_id, verification_hash, verified_at
		FROM por.auditor_verifications
		WHERE attestation_id = $1
		ORDER BY verified_at DESC`,
		attestationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list auditor verifications: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditorVerification
	for rows.Next() {
		var v domain.AuditorVerification
		var verifiedAt time.Time
		if err := rows.Scan(&v.ID, &v.AttestationID, &v.AuditorName, &v.AuditorID, &v.VerificationHash, &verifiedAt); err != nil {
			return nil, fmt.Errorf("scan auditor verification: %w", err)
		}
		v.VerifiedAt = verifiedAt
		out = append(out, v)
	}
	return out, rows.Err()
}
