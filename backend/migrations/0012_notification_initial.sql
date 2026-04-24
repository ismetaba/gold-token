-- +goose Up

CREATE SCHEMA IF NOT EXISTS notification;

CREATE TABLE notification.templates (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type       TEXT        NOT NULL UNIQUE,
    subject_template TEXT        NOT NULL,
    body_template    TEXT        NOT NULL,
    channels         TEXT[]      NOT NULL DEFAULT '{inapp}',
    active           BOOLEAN     NOT NULL DEFAULT true
);

-- Seed default templates.
INSERT INTO notification.templates (event_type, subject_template, body_template, channels)
VALUES
    ('kyc.approved', 'KYC Approved', 'Your KYC application has been approved. You can now trade gold tokens.', '{inapp,email}'),
    ('kyc.rejected', 'KYC Rejected', 'Your KYC application was not approved. Please contact support for details.', '{inapp,email}'),
    ('gold.order.created.v1', 'Order Created', 'Your order {{order_id}} has been created and is being processed.', '{inapp}'),
    ('gold.mint.executed.v1', 'Tokens Minted', 'Your gold tokens have been minted successfully.', '{inapp,email}'),
    ('gold.burn.executed.v1', 'Tokens Redeemed', 'Your gold token redemption has been processed successfully.', '{inapp,email}'),
    ('gold.compliance.alert.v1', 'Compliance Alert', 'A compliance review has been flagged on your account. Please contact support.', '{inapp,email}');

CREATE TABLE notification.deliveries (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL,
    template_id UUID        REFERENCES notification.templates(id),
    channel     TEXT        NOT NULL,
    subject     TEXT        NOT NULL DEFAULT '',
    body        TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL DEFAULT 'pending',
    error       TEXT        NOT NULL DEFAULT '',
    sent_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notification_deliveries_user ON notification.deliveries (user_id, created_at DESC);

CREATE TABLE notification.preferences (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL UNIQUE,
    email_enabled   BOOLEAN     NOT NULL DEFAULT true,
    webhook_url     TEXT        NOT NULL DEFAULT '',
    webhook_enabled BOOLEAN     NOT NULL DEFAULT false,
    inapp_enabled   BOOLEAN     NOT NULL DEFAULT true,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE IF EXISTS notification.preferences;
DROP TABLE IF EXISTS notification.deliveries;
DROP TABLE IF EXISTS notification.templates;
DROP SCHEMA IF EXISTS notification;
