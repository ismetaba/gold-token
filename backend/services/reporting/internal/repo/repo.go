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

type MaterializedRepo interface {
	// IncrementTransactionCounter upserts a daily transaction counter row.
	// kind is "mint" or "burn"; amountWei is the wei amount as a decimal string.
	IncrementTransactionCounter(ctx context.Context, period, kind, amountWei string) error
	// UpsertReserveSnapshot upserts a daily reserve snapshot.
	UpsertReserveSnapshot(ctx context.Context, period, goldBalanceWei, tokenSupplyWei string, generatedAt time.Time) error
}

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

// ── Materialized repo ──────────────────────────────────────────────────────

type pgMaterializedRepo struct{ pool *pgxpool.Pool }

func NewPGMaterializedRepo(pool *pgxpool.Pool) MaterializedRepo {
	return &pgMaterializedRepo{pool: pool}
}

func (r *pgMaterializedRepo) IncrementTransactionCounter(ctx context.Context, period, kind, amountWei string) error {
	// Upsert a JSONB row that tracks mint_count, burn_count, mint_volume_wei, burn_volume_wei.
	// We use a numeric cast and jsonb merge so concurrent events accumulate safely.
	var mintCountDelta, burnCountDelta int
	var mintVolumeDelta, burnVolumeDelta string
	if kind == "mint" {
		mintCountDelta = 1
		mintVolumeDelta = amountWei
		burnVolumeDelta = "0"
	} else {
		burnCountDelta = 1
		burnVolumeDelta = amountWei
		mintVolumeDelta = "0"
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO reporting.materialized_reports (report_type, period, data, generated_at)
		VALUES ('transactions', $1,
			jsonb_build_object(
				'mint_count',      $2::int,
				'burn_count',      $3::int,
				'mint_volume_wei', $4::text,
				'burn_volume_wei', $5::text
			), now())
		ON CONFLICT (report_type, period) DO UPDATE
		SET data = jsonb_build_object(
				'mint_count',      COALESCE((reporting.materialized_reports.data->>'mint_count')::bigint,0) + $2::int,
				'burn_count',      COALESCE((reporting.materialized_reports.data->>'burn_count')::bigint,0) + $3::int,
				'mint_volume_wei', (COALESCE((reporting.materialized_reports.data->>'mint_volume_wei')::numeric,0) + $4::numeric)::text,
				'burn_volume_wei', (COALESCE((reporting.materialized_reports.data->>'burn_volume_wei')::numeric,0) + $5::numeric)::text
			),
		    generated_at = now()`,
		period, mintCountDelta, burnCountDelta, mintVolumeDelta, burnVolumeDelta,
	)
	if err != nil {
		return fmt.Errorf("upsert transaction counter: %w", err)
	}
	return nil
}

func (r *pgMaterializedRepo) UpsertReserveSnapshot(ctx context.Context, period, goldBalanceWei, tokenSupplyWei string, generatedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO reporting.materialized_reports (report_type, period, data, generated_at)
		VALUES ('reserves', $1,
			jsonb_build_object(
				'gold_balance_wei',  $2::text,
				'token_supply_wei',  $3::text,
				'attestation_count', 1
			), $4)
		ON CONFLICT (report_type, period) DO UPDATE
		SET data = jsonb_build_object(
				'gold_balance_wei',  $2::text,
				'token_supply_wei',  $3::text,
				'attestation_count', COALESCE((reporting.materialized_reports.data->>'attestation_count')::int, 0) + 1
			),
		    generated_at = $4`,
		period, goldBalanceWei, tokenSupplyWei, generatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert reserve snapshot: %w", err)
	}
	return nil
}
