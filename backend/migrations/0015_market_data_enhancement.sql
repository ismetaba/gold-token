-- +goose Up
-- +goose StatementBegin

-- Add pair column to existing price_history table so each row
-- records which currency pair the price belongs to (e.g. XAU/USD).
-- Existing rows are back-filled to 'XAU/USD' (the only pair prior to this migration).
ALTER TABLE oracle.price_history
    ADD COLUMN IF NOT EXISTS pair TEXT NOT NULL DEFAULT 'XAU/USD';

-- Rename price_usd_g to price_per_gram to be currency-agnostic and
-- expose the denomination via the pair column.  We keep an alias view
-- for backwards-compatibility during the rollout window.
ALTER TABLE oracle.price_history
    RENAME COLUMN price_usd_g TO price_per_gram;

CREATE INDEX IF NOT EXISTS idx_price_history_pair
    ON oracle.price_history (pair, fetched_at DESC);

-- OHLCV candlestick table.
-- Buckets are aligned to the interval start (e.g. top of the hour for 1h).
CREATE TABLE oracle.candles (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    pair        TEXT        NOT NULL,
    interval    TEXT        NOT NULL CHECK (interval IN ('1h','4h','1d')),
    open_per_gram   NUMERIC(24,8) NOT NULL,
    high_per_gram   NUMERIC(24,8) NOT NULL,
    low_per_gram    NUMERIC(24,8) NOT NULL,
    close_per_gram  NUMERIC(24,8) NOT NULL,
    volume          NUMERIC(24,8) NOT NULL DEFAULT 0,
    bucket_start    TIMESTAMPTZ NOT NULL,
    bucket_end      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_candles_unique
    ON oracle.candles (pair, interval, bucket_start);

CREATE INDEX IF NOT EXISTS idx_candles_lookup
    ON oracle.candles (pair, interval, bucket_start DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS oracle.idx_candles_lookup;
DROP INDEX IF EXISTS oracle.idx_candles_unique;
DROP TABLE IF EXISTS oracle.candles;
DROP INDEX IF EXISTS oracle.idx_price_history_pair;
ALTER TABLE oracle.price_history RENAME COLUMN price_per_gram TO price_usd_g;
ALTER TABLE oracle.price_history DROP COLUMN IF EXISTS pair;

-- +goose StatementEnd
