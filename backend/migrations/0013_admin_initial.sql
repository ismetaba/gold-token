-- +goose Up
CREATE SCHEMA IF NOT EXISTS admin;

CREATE TABLE admin.users (
    id          UUID        PRIMARY KEY,
    email       TEXT        NOT NULL UNIQUE,
    password_hash TEXT      NOT NULL,
    role        TEXT        NOT NULL DEFAULT 'viewer'
                            CHECK (role IN ('super_admin','ops','compliance_viewer','viewer')),
    active      BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE admin.api_keys (
    id          UUID        PRIMARY KEY,
    admin_user_id UUID     NOT NULL REFERENCES admin.users(id),
    key_hash    TEXT        NOT NULL UNIQUE,
    name        TEXT        NOT NULL,
    scopes      TEXT[]      NOT NULL DEFAULT '{}',
    last_used_at TIMESTAMPTZ,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_admin_api_keys_user ON admin.api_keys(admin_user_id);

CREATE TABLE admin.sessions (
    id          UUID        PRIMARY KEY,
    admin_user_id UUID     NOT NULL REFERENCES admin.users(id),
    token_hash  TEXT        NOT NULL UNIQUE,
    ip_address  TEXT        NOT NULL DEFAULT '',
    user_agent  TEXT        NOT NULL DEFAULT '',
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_admin_sessions_user ON admin.sessions(admin_user_id);
CREATE INDEX idx_admin_sessions_expires ON admin.sessions(expires_at);

-- +goose Down
DROP TABLE IF EXISTS admin.sessions;
DROP TABLE IF EXISTS admin.api_keys;
DROP TABLE IF EXISTS admin.users;
DROP SCHEMA IF EXISTS admin;
