-- Schema for: profiles, categories, receipts, receipt_files, extract_job
-- UUIDs via pgcrypto.gen_random_uuid()

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =========================
-- profiles (multi-business)
-- =========================
CREATE TABLE IF NOT EXISTS profiles
(
    id               uuid PRIMARY KEY     DEFAULT gen_random_uuid(),
    name             text        NOT NULL UNIQUE,
    job_title       text,
    job_description text,
    default_currency char(3)     NOT NULL DEFAULT 'USD',
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- ==================================== -- receipt_files (ingested file artifact) -- ====================================
CREATE TABLE IF NOT EXISTS receipt_files
(
    id           uuid PRIMARY KEY     DEFAULT gen_random_uuid(),
    filename     text        NOT NULL, -- original filename (basename of source_path)
    file_ext     text        NOT NULL, -- 'pdf','jpg','png',...
    file_size    integer     NOT NULL,
    source_path  text        NOT NULL, -- original absolute/virtual path as seen by the app
    profile_id   uuid        NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    content_hash bytea       NOT NULL, -- sha256(file bytes)
    uploaded_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (profile_id, content_hash)  -- dedupe per profile
);

CREATE INDEX IF NOT EXISTS idx_files_uploaded_at ON receipt_files (profile_id, uploaded_at DESC);

-- =========================
-- receipts (normalized fact)
-- =========================
CREATE TABLE IF NOT EXISTS receipts
(
    merchant_name text           NOT NULL,
    tx_date       date           NOT NULL,
    total         numeric(12, 2) NOT NULL,
    subtotal      numeric(12, 2),
    tax           numeric(12, 2),
    category_name text           NOT NULL,
    description   text,
    file_path     text,
    id            uuid PRIMARY KEY        DEFAULT gen_random_uuid(),
    profile_id    uuid           NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    file_id       uuid REFERENCES receipt_files (id) ON DELETE RESTRICT,
    currency_code char(3)        NOT NULL,
    created_at    timestamptz    NOT NULL DEFAULT now(),
    updated_at    timestamptz    NOT NULL DEFAULT now(),
    is_current    boolean        NOT NULL DEFAULT true
);

-- Helpful lookups
CREATE INDEX IF NOT EXISTS idx_receipts_profile_date ON receipts (profile_id, tx_date);
CREATE INDEX IF NOT EXISTS idx_receipts_category_name ON receipts (profile_id, category_name);
CREATE INDEX IF NOT EXISTS idx_receipts_merchant ON receipts (merchant_name);

-- ==============================
-- extract_job (processing runs)
-- ==============================
CREATE TABLE IF NOT EXISTS extract_job
(
    id                    uuid PRIMARY KEY     DEFAULT gen_random_uuid(),

    -- ownership/links
    file_id               uuid        NOT NULL REFERENCES receipt_files (id) ON DELETE CASCADE,
    profile_id            uuid        NOT NULL REFERENCES profiles (id) ON DELETE RESTRICT,
    receipt_id            uuid        REFERENCES receipts (id) ON DELETE SET NULL,

    -- processing metadata
    format                text        NOT NULL CHECK (format IN ('PDF', 'IMAGE', 'TXT')),
    started_at            timestamptz NOT NULL DEFAULT now(),
    finished_at           timestamptz,
    status                text, -- e.g., 'QUEUED','RUNNING','SUCCEEDED','FAILED'
    error_message         text,

    -- model outputs
    extraction_confidence real,
    needs_review          boolean     NOT NULL DEFAULT false,
    ocr_text              text,
    extracted_json        jsonb,
    model_name            text,
    model_params          jsonb
);

CREATE INDEX IF NOT EXISTS idx_job_profile_status_started ON extract_job (profile_id, status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_file ON extract_job (file_id);
CREATE INDEX IF NOT EXISTS idx_job_receipt ON extract_job (receipt_id);

COMMIT;
