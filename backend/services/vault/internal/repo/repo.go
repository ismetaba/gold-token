// Package repo provides PostgreSQL-backed Vault Integration persistence.
package repo

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/vault/internal/domain"
)

var (
	ErrNotFound     = errors.New("record not found")
	ErrDuplicateBar = errors.New("bar already exists")
	ErrBarAllocated = errors.New("bar is currently allocated")
)

// BarRepo manages gold bar inventory (reads/writes mint.gold_bars).
type BarRepo interface {
	Ingest(ctx context.Context, bar domain.GoldBar) error
	BySerial(ctx context.Context, serial string) (domain.GoldBar, error)
	List(ctx context.Context, vaultID *uuid.UUID, status *string, limit, offset int) ([]domain.GoldBar, error)
	UpdateVault(ctx context.Context, serial string, newVaultID uuid.UUID) error
}

// MovementRepo tracks bar movements.
type MovementRepo interface {
	Create(ctx context.Context, m domain.BarMovement) error
	ListByBar(ctx context.Context, serial string, limit int) ([]domain.BarMovement, error)
}

// AuditRepo stores vault audit records.
type AuditRepo interface {
	Create(ctx context.Context, a domain.AuditRecord) error
	List(ctx context.Context, vaultID *uuid.UUID, limit, offset int) ([]domain.AuditRecord, error)
}

// VaultRepo reads vault info from mint.vaults.
type VaultRepo interface {
	ByID(ctx context.Context, id uuid.UUID) (domain.Vault, error)
}

// ── PostgreSQL implementations ─────────────────────────────────────────────

type pgBarRepo struct{ pool *pgxpool.Pool }

func NewPGBarRepo(pool *pgxpool.Pool) BarRepo { return &pgBarRepo{pool: pool} }

func (r *pgBarRepo) Ingest(ctx context.Context, bar domain.GoldBar) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO mint.gold_bars
			(serial_no, vault_id, weight_grams_wei, allocated_sum_wei, purity_9999, refiner_lbma_id, cast_date, status, ingested_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		bar.SerialNo, bar.VaultID, bar.WeightGramsWei.String(), "0",
		bar.Purity9999, bar.RefinerLBMAID, bar.CastDate,
		string(domain.BarAvailable), bar.IngestedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return ErrDuplicateBar
		}
		return fmt.Errorf("ingest bar: %w", err)
	}
	return nil
}

func (r *pgBarRepo) BySerial(ctx context.Context, serial string) (domain.GoldBar, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT serial_no, vault_id, weight_grams_wei, allocated_sum_wei, purity_9999, refiner_lbma_id, cast_date, status, ingested_at
		 FROM mint.gold_bars WHERE serial_no = $1`, serial)

	var (
		bar          domain.GoldBar
		weightStr    string
		allocStr     string
		statusStr    string
	)
	err := row.Scan(
		&bar.SerialNo, &bar.VaultID, &weightStr, &allocStr,
		&bar.Purity9999, &bar.RefinerLBMAID, &bar.CastDate,
		&statusStr, &bar.IngestedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.GoldBar{}, ErrNotFound
		}
		return domain.GoldBar{}, fmt.Errorf("query bar: %w", err)
	}
	bar.WeightGramsWei = mustParseBigInt(weightStr)
	bar.AllocatedSumWei = mustParseBigInt(allocStr)
	bar.Status = domain.BarStatus(statusStr)
	return bar, nil
}

func (r *pgBarRepo) List(ctx context.Context, vaultID *uuid.UUID, status *string, limit, offset int) ([]domain.GoldBar, error) {
	q := `SELECT serial_no, vault_id, weight_grams_wei, allocated_sum_wei, purity_9999, refiner_lbma_id, cast_date, status, ingested_at
	      FROM mint.gold_bars WHERE 1=1`
	args := []any{}
	idx := 1

	if vaultID != nil {
		q += fmt.Sprintf(" AND vault_id = $%d", idx)
		args = append(args, *vaultID)
		idx++
	}
	if status != nil {
		q += fmt.Sprintf(" AND status = $%d", idx)
		args = append(args, *status)
		idx++
	}
	q += fmt.Sprintf(" ORDER BY ingested_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list bars: %w", err)
	}
	defer rows.Close()

	var out []domain.GoldBar
	for rows.Next() {
		var (
			bar       domain.GoldBar
			weightStr string
			allocStr  string
			statusStr string
		)
		if err := rows.Scan(
			&bar.SerialNo, &bar.VaultID, &weightStr, &allocStr,
			&bar.Purity9999, &bar.RefinerLBMAID, &bar.CastDate,
			&statusStr, &bar.IngestedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bar: %w", err)
		}
		bar.WeightGramsWei = mustParseBigInt(weightStr)
		bar.AllocatedSumWei = mustParseBigInt(allocStr)
		bar.Status = domain.BarStatus(statusStr)
		out = append(out, bar)
	}
	return out, rows.Err()
}

func (r *pgBarRepo) UpdateVault(ctx context.Context, serial string, newVaultID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE mint.gold_bars SET vault_id = $2 WHERE serial_no = $1 AND status = 'available'`,
		serial, newVaultID)
	if err != nil {
		return fmt.Errorf("update bar vault: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Check if bar exists but is allocated.
		var exists bool
		_ = r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM mint.gold_bars WHERE serial_no = $1)`, serial).Scan(&exists)
		if !exists {
			return ErrNotFound
		}
		return ErrBarAllocated
	}
	return nil
}

// ── Movement repo ──────────────────────────────────────────────────────────

type pgMovementRepo struct{ pool *pgxpool.Pool }

func NewPGMovementRepo(pool *pgxpool.Pool) MovementRepo { return &pgMovementRepo{pool: pool} }

func (r *pgMovementRepo) Create(ctx context.Context, m domain.BarMovement) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO vault.bar_movements
			(id, bar_serial, from_vault_id, to_vault_id, movement_type, initiated_by, reason, moved_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		m.ID, m.BarSerial, m.FromVaultID, m.ToVaultID,
		m.Type, m.InitiatedBy, m.Reason, m.MovedAt,
	)
	if err != nil {
		return fmt.Errorf("insert movement: %w", err)
	}
	return nil
}

func (r *pgMovementRepo) ListByBar(ctx context.Context, serial string, limit int) ([]domain.BarMovement, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, bar_serial, from_vault_id, to_vault_id, movement_type, initiated_by, reason, moved_at
		 FROM vault.bar_movements WHERE bar_serial = $1 ORDER BY moved_at DESC LIMIT $2`,
		serial, limit)
	if err != nil {
		return nil, fmt.Errorf("list movements: %w", err)
	}
	defer rows.Close()

	var out []domain.BarMovement
	for rows.Next() {
		var m domain.BarMovement
		if err := rows.Scan(
			&m.ID, &m.BarSerial, &m.FromVaultID, &m.ToVaultID,
			&m.Type, &m.InitiatedBy, &m.Reason, &m.MovedAt,
		); err != nil {
			return nil, fmt.Errorf("scan movement: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ── Audit repo ─────────────────────────────────────────────────────────────

type pgAuditRepo struct{ pool *pgxpool.Pool }

func NewPGAuditRepo(pool *pgxpool.Pool) AuditRepo { return &pgAuditRepo{pool: pool} }

func (r *pgAuditRepo) Create(ctx context.Context, a domain.AuditRecord) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO vault.audit_records
			(id, vault_id, auditor, audit_type, bar_count, total_weight_grams_wei, discrepancies, status, audited_at, recorded_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		a.ID, a.VaultID, a.Auditor, a.AuditType,
		a.BarCount, a.TotalWeightWei.String(), a.Discrepancies,
		a.Status, a.AuditedAt, a.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit record: %w", err)
	}
	return nil
}

func (r *pgAuditRepo) List(ctx context.Context, vaultID *uuid.UUID, limit, offset int) ([]domain.AuditRecord, error) {
	q := `SELECT id, vault_id, auditor, audit_type, bar_count, total_weight_grams_wei, discrepancies, status, audited_at, recorded_at
	      FROM vault.audit_records`
	args := []any{}
	idx := 1

	if vaultID != nil {
		q += fmt.Sprintf(" WHERE vault_id = $%d", idx)
		args = append(args, *vaultID)
		idx++
	}
	q += fmt.Sprintf(" ORDER BY audited_at DESC LIMIT $%d OFFSET $%d", idx, idx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit records: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditRecord
	for rows.Next() {
		var (
			a        domain.AuditRecord
			weightStr string
		)
		if err := rows.Scan(
			&a.ID, &a.VaultID, &a.Auditor, &a.AuditType,
			&a.BarCount, &weightStr, &a.Discrepancies,
			&a.Status, &a.AuditedAt, &a.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit record: %w", err)
		}
		a.TotalWeightWei = mustParseBigInt(weightStr)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ── Vault repo ─────────────────────────────────────────────────────────────

type pgVaultRepo struct{ pool *pgxpool.Pool }

func NewPGVaultRepo(pool *pgxpool.Pool) VaultRepo { return &pgVaultRepo{pool: pool} }

func (r *pgVaultRepo) ByID(ctx context.Context, id uuid.UUID) (domain.Vault, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, code, arena, operator, address, country_code, lbma_approved, insured_by
		 FROM mint.vaults WHERE id = $1`, id)

	var v domain.Vault
	err := row.Scan(&v.ID, &v.Code, &v.Arena, &v.Operator, &v.Address, &v.CountryCode, &v.LBMAApproved, &v.InsuredBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Vault{}, ErrNotFound
		}
		return domain.Vault{}, fmt.Errorf("query vault: %w", err)
	}
	return v, nil
}

// ── helpers ────────────────────────────────────────────────────────────────

func mustParseBigInt(s string) *big.Int {
	n := new(big.Int)
	n.SetString(s, 10)
	return n
}

func isDuplicateKey(err error) bool {
	return err != nil && (errors.Is(err, errors.New("duplicate key")) ||
		// pgx wraps unique violation as code 23505.
		fmt.Sprintf("%v", err) != "" && containsUniqueViolation(err))
}

func containsUniqueViolation(err error) bool {
	s := err.Error()
	return len(s) > 0 && (contains(s, "23505") || contains(s, "duplicate key"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure time is imported.
var _ = time.Now
