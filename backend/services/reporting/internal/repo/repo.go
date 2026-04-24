package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/reporting/internal/domain"
)

var ErrNotFound = errors.New("record not found")

type ReportJobRepo interface {
	Create(ctx context.Context, j domain.ReportJob) error
	ByID(ctx context.Context, id uuid.UUID) (domain.ReportJob, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status, errMsg string, completedAt *time.Time) error
}

type QueryRepo interface {
	TransactionSummary(ctx context.Context, from, to time.Time) ([]domain.TransactionSummary, error)
	ReserveSummary(ctx context.Context, from, to time.Time) ([]domain.ReserveSummary, error)
	ComplianceSummary(ctx context.Context) (domain.ComplianceSummary, error)
}

// ── Report Job repo ────────────────────────────────────────────────────────

type pgReportJobRepo struct{ pool *pgxpool.Pool }

func NewPGReportJobRepo(pool *pgxpool.Pool) ReportJobRepo { return &pgReportJobRepo{pool: pool} }

func (r *pgReportJobRepo) Create(ctx context.Context, j domain.ReportJob) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO reporting.report_jobs
			(id, report_type, parameters, status, output_path, error, requested_by, started_at, completed_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		j.ID, j.ReportType, j.Parameters, j.Status, j.OutputPath,
		j.Error, j.RequestedBy, j.StartedAt, j.CompletedAt, j.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert report job: %w", err)
	}
	return nil
}

func (r *pgReportJobRepo) ByID(ctx context.Context, id uuid.UUID) (domain.ReportJob, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, report_type, parameters, status, output_path, error, requested_by, started_at, completed_at, created_at
		 FROM reporting.report_jobs WHERE id = $1`, id)

	var j domain.ReportJob
	err := row.Scan(
		&j.ID, &j.ReportType, &j.Parameters, &j.Status, &j.OutputPath,
		&j.Error, &j.RequestedBy, &j.StartedAt, &j.CompletedAt, &j.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ReportJob{}, ErrNotFound
		}
		return domain.ReportJob{}, fmt.Errorf("query report job: %w", err)
	}
	return j, nil
}

func (r *pgReportJobRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status, errMsg string, completedAt *time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reporting.report_jobs SET status = $2, error = $3, completed_at = $4 WHERE id = $1`,
		id, status, errMsg, completedAt)
	if err != nil {
		return fmt.Errorf("update report job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ── Query repo (cross-schema reads) ────────────────────────────────────────

type pgQueryRepo struct{ pool *pgxpool.Pool }

func NewPGQueryRepo(pool *pgxpool.Pool) QueryRepo { return &pgQueryRepo{pool: pool} }

func (r *pgQueryRepo) TransactionSummary(ctx context.Context, from, to time.Time) ([]domain.TransactionSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT
			DATE(created_at) AS date,
			COUNT(*) FILTER (WHERE type = 'buy') AS mint_count,
			COUNT(*) FILTER (WHERE type = 'sell') AS burn_count,
			COALESCE(SUM(CAST(amount_wei AS NUMERIC)) FILTER (WHERE type = 'buy'), 0) AS mint_volume,
			COALESCE(SUM(CAST(amount_wei AS NUMERIC)) FILTER (WHERE type = 'sell'), 0) AS burn_volume,
			0 AS fee_volume
		 FROM orders.orders
		 WHERE created_at >= $1 AND created_at <= $2
		 GROUP BY DATE(created_at)
		 ORDER BY date`, from, to)
	if err != nil {
		return nil, fmt.Errorf("transaction summary: %w", err)
	}
	defer rows.Close()

	var out []domain.TransactionSummary
	for rows.Next() {
		var s domain.TransactionSummary
		if err := rows.Scan(&s.Date, &s.MintCount, &s.BurnCount, &s.MintVolumeWei, &s.BurnVolumeWei, &s.FeeVolumeWei); err != nil {
			return nil, fmt.Errorf("scan tx summary: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *pgQueryRepo) ReserveSummary(ctx context.Context, from, to time.Time) ([]domain.ReserveSummary, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT
			DATE(reconciled_at) AS date,
			COALESCE(actual_balance_wei::TEXT, '0') AS gold_balance,
			'0' AS token_supply,
			0 AS attestation_count
		 FROM treasury.reconciliation_logs
		 WHERE reconciled_at >= $1 AND reconciled_at <= $2
		 ORDER BY date`, from, to)
	if err != nil {
		return nil, fmt.Errorf("reserve summary: %w", err)
	}
	defer rows.Close()

	var out []domain.ReserveSummary
	for rows.Next() {
		var s domain.ReserveSummary
		if err := rows.Scan(&s.Date, &s.GoldBalanceWei, &s.TokenSupplyWei, &s.AttestationCount); err != nil {
			return nil, fmt.Errorf("scan reserve summary: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *pgQueryRepo) ComplianceSummary(ctx context.Context) (domain.ComplianceSummary, error) {
	var s domain.ComplianceSummary

	// Compliance screenings.
	_ = r.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status = 'approved') AS approved,
			COUNT(*) FILTER (WHERE status = 'rejected') AS rejected
		 FROM compliance.compliance_checks`).Scan(&s.TotalScreenings, &s.ApprovedCount, &s.RejectedCount)

	// KYC applications.
	_ = r.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE status = 'pending' OR status = 'under_review') AS pending,
			COUNT(*) FILTER (WHERE status = 'approved') AS approved,
			COUNT(*) FILTER (WHERE status = 'rejected') AS rejected
		 FROM kyc.applications`).Scan(&s.PendingKYC, &s.ApprovedKYC, &s.RejectedKYC)

	return s, nil
}
