-- +goose Up
CREATE SCHEMA IF NOT EXISTS reporting;

CREATE TABLE reporting.report_jobs (
    id            UUID          PRIMARY KEY,
    report_type   TEXT          NOT NULL CHECK (report_type IN ('transactions','reserves','compliance')),
    parameters    JSONB,
    status        TEXT          NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','running','completed','failed')),
    output_path   TEXT          NOT NULL DEFAULT '',
    error         TEXT          NOT NULL DEFAULT '',
    requested_by  TEXT          NOT NULL DEFAULT '',
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_report_jobs_status ON reporting.report_jobs(status);
CREATE INDEX idx_report_jobs_created ON reporting.report_jobs(created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS reporting.report_jobs;
DROP SCHEMA IF EXISTS reporting;
