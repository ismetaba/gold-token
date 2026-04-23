-- Price oracle service schema
CREATE SCHEMA IF NOT EXISTS oracle;

CREATE TABLE oracle.price_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    price_usd_g NUMERIC(18,8) NOT NULL,  -- USD per gram (24-karat gold)
    provider    TEXT NOT NULL,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_price_history_fetched ON oracle.price_history(fetched_at DESC);
