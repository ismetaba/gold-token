-- +goose Up
-- +goose StatementBegin
CREATE SCHEMA IF NOT EXISTS kyc;

CREATE TYPE kyc.application_status AS ENUM (
    'pending',
    'under_review',
    'approved',
    'rejected'
);

CREATE TABLE kyc.applications (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL,
    status          kyc.application_status NOT NULL DEFAULT 'pending',
    document_path   TEXT NOT NULL,
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    date_of_birth   DATE NOT NULL,
    nationality     TEXT NOT NULL,
    reviewer_note   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at     TIMESTAMPTZ
);

-- One active application per user (pending or under_review)
CREATE UNIQUE INDEX uq_kyc_applications_user_active
    ON kyc.applications (user_id)
    WHERE status IN ('pending', 'under_review');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS kyc.applications;
DROP TYPE IF EXISTS kyc.application_status;
DROP SCHEMA IF EXISTS kyc CASCADE;
-- +goose StatementEnd
