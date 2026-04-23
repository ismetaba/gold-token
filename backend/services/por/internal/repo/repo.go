// Package repo provides PostgreSQL-backed persistence for the PoR service.
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/por/internal/domain"
)

var ErrNotFound = errors.New("por: not found")

// AttestationRepo persists and queries attestation records.
type AttestationRepo interface {
	// Create inserts a new attestation record.
	// Idempotent: silently ignores duplicates keyed on timestamp_sec.
	Create(ctx context.Context, a domain.Attestation) error

	// Latest returns the most recently recorded attestation (by timestamp_sec).
	Latest(ctx context.Context) (domain.Attestation, error)

	// List returns attestation records ordered by timestamp_sec descending.
	// limit: max rows; offset: pagination offset.
	List(ctx context.Context, limit, offset int) ([]domain.Attestation, error)
}

type pgAttestationRepo struct{ pool *pgxpool.Pool }

// NewPGAttestationRepo returns a PostgreSQL-backed AttestationRepo.
func NewPGAttestationRepo(pool *pgxpool.Pool) AttestationRepo {
	return &pgAttestationRepo{pool: pool}
}

func (r *pgAttestationRepo) Create(ctx context.Context, a domain.Attestation) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO por.attestation_log
		  (id, on_chain_idx, timestamp_sec, as_of_sec, total_grams_wei,
		   merkle_root, ipfs_cid, auditor, tx_hash, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (timestamp_sec) DO NOTHING`,
		a.ID,
		a.OnChainIdx,
		a.TimestampSec,
		a.AsOfSec,
		a.TotalGramsWei,
		a.MerkleRoot,
		a.IPFSCid,
		a.Auditor,
		nullableString(a.TxHash),
		a.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("insert attestation: %w", err)
	}
	return nil
}

func (r *pgAttestationRepo) Latest(ctx context.Context) (domain.Attestation, error) {
	return r.queryOne(ctx, `
		SELECT id, on_chain_idx, timestamp_sec, as_of_sec, total_grams_wei,
		       merkle_root, ipfs_cid, auditor, tx_hash, recorded_at
		FROM por.attestation_log
		ORDER BY timestamp_sec DESC
		LIMIT 1`)
}

func (r *pgAttestationRepo) List(ctx context.Context, limit, offset int) ([]domain.Attestation, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, on_chain_idx, timestamp_sec, as_of_sec, total_grams_wei,
		       merkle_root, ipfs_cid, auditor, tx_hash, recorded_at
		FROM por.attestation_log
		ORDER BY timestamp_sec DESC
		LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list attestations: %w", err)
	}
	defer rows.Close()

	var out []domain.Attestation
	for rows.Next() {
		a, err := scanAttestation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan attestation: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func (r *pgAttestationRepo) queryOne(ctx context.Context, q string, args ...interface{}) (domain.Attestation, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return domain.Attestation{}, fmt.Errorf("query attestation: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.Attestation{}, fmt.Errorf("query attestation: %w", err)
		}
		return domain.Attestation{}, ErrNotFound
	}
	return scanAttestation(rows)
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanAttestation(row scanner) (domain.Attestation, error) {
	var a domain.Attestation
	var txHash *string
	var recordedAt time.Time
	if err := row.Scan(
		&a.ID,
		&a.OnChainIdx,
		&a.TimestampSec,
		&a.AsOfSec,
		&a.TotalGramsWei,
		&a.MerkleRoot,
		&a.IPFSCid,
		&a.Auditor,
		&txHash,
		&recordedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Attestation{}, ErrNotFound
		}
		return domain.Attestation{}, err
	}
	a.RecordedAt = recordedAt
	if txHash != nil {
		a.TxHash = *txHash
	}
	return a, nil
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// NewID generates a new UUID v7 for a new attestation record.
func NewID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}
