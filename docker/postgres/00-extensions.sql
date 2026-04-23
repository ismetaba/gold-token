-- Enable pgcrypto for gen_random_uuid() (PostgreSQL 16 has it built-in via pg_catalog)
-- This is a no-op on PG16 but kept for clarity.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
