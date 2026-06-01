-- 0001_init.up.sql
--
-- M3 schema: ingestion (pending uploads) + library (canonical documents,
-- artifacts). Per ADR 0006 we use one schema per bounded context. Cross-
-- schema joins are not allowed; foreign keys across schemas are fine.
--
-- Conventions
--   * UUIDs are primary keys (gen_random_uuid()).
--   * `owner_id` is the Clerk userId from the JWT (string, not a row).
--     v0.1 has no users table — Identity lives in the auth provider.
--   * Timestamps are `timestamptz` with default `now()`.
--   * Money columns and floats are not appropriate here yet; confidence is
--     a 0..100 integer to avoid float comparisons.

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS ingestion;
CREATE SCHEMA IF NOT EXISTS library;
-- Reserve schemas owned by later milestones so role grants and FK refs
-- have stable targets.
CREATE SCHEMA IF NOT EXISTS extraction;
CREATE SCHEMA IF NOT EXISTS search;

-- ----------------------------------------------------------------------
-- library.documents — canonical record per uploaded document.
-- ----------------------------------------------------------------------
CREATE TYPE library.document_status AS ENUM (
    'queued',
    'extracting',
    'ready',
    'failed',
    'deleted'
);

CREATE TABLE library.documents (
    id               uuid                    PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id         text                    NOT NULL,
    title            text                    NOT NULL,
    source_filename  text                    NOT NULL,
    sha256           text                    NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    byte_size        bigint                  NOT NULL CHECK (byte_size > 0),
    mime_type        text                    NOT NULL,
    status           library.document_status NOT NULL DEFAULT 'queued',
    page_count       integer                 NULL,
    word_count       integer                 NULL,
    confidence       integer                 NULL CHECK (confidence BETWEEN 0 AND 100),
    last_error       text                    NULL,
    raw_object_key   text                    NOT NULL,
    created_at       timestamptz             NOT NULL DEFAULT now(),
    updated_at       timestamptz             NOT NULL DEFAULT now()
);

-- Property 6: (ownerId, sha256) is unique. Re-upload returns existing row.
-- Excludes soft-deleted rows so a user can re-upload after deletion.
CREATE UNIQUE INDEX documents_owner_sha256_alive_uk
    ON library.documents (owner_id, sha256)
    WHERE status <> 'deleted';

-- Owner-scoped list queries hit this every page.
CREATE INDEX documents_owner_created_at_idx
    ON library.documents (owner_id, created_at DESC)
    WHERE status <> 'deleted';

-- ----------------------------------------------------------------------
-- library.artifacts — derived files per document. Populated in M4+.
-- ----------------------------------------------------------------------
CREATE TYPE library.artifact_kind AS ENUM (
    'markdown',
    'metadata',
    'thumbnail',
    'page-text'
);

CREATE TABLE library.artifacts (
    id           uuid                  PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id  uuid                  NOT NULL REFERENCES library.documents (id) ON DELETE CASCADE,
    kind         library.artifact_kind NOT NULL,
    object_key   text                  NOT NULL,
    byte_size    bigint                NOT NULL CHECK (byte_size >= 0),
    created_at   timestamptz           NOT NULL DEFAULT now(),
    UNIQUE (document_id, kind)
);

CREATE INDEX artifacts_document_id_idx ON library.artifacts (document_id);

-- ----------------------------------------------------------------------
-- ingestion.uploads — pending uploads awaiting bytes via presigned PUT.
--
-- A row is created when the client calls POST /v1/uploads. It is removed
-- (or the document_id is set) once the user calls /finalize. Rows older
-- than `expires_at` whose object never landed are sweep targets.
-- ----------------------------------------------------------------------
CREATE TYPE ingestion.upload_status AS ENUM (
    'pending',    -- waiting for the client's PUT
    'finalized',  -- /finalize succeeded; document_id is set
    'expired'     -- swept by the cron after 24h with no PUT
);

CREATE TABLE ingestion.uploads (
    id              uuid                    PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        text                    NOT NULL,
    document_id     uuid                    NULL REFERENCES library.documents (id) ON DELETE SET NULL,
    object_key      text                    NOT NULL,
    bucket          text                    NOT NULL,
    sha256          text                    NOT NULL CHECK (sha256 ~ '^[a-f0-9]{64}$'),
    mime_type       text                    NOT NULL,
    byte_size       bigint                  NOT NULL CHECK (byte_size > 0),
    source_filename text                    NOT NULL,
    title           text                    NOT NULL,
    status          ingestion.upload_status NOT NULL DEFAULT 'pending',
    expires_at      timestamptz             NOT NULL,
    created_at      timestamptz             NOT NULL DEFAULT now(),
    finalized_at    timestamptz             NULL
);

-- Sweeps look for stale pending rows.
CREATE INDEX uploads_status_expires_at_idx
    ON ingestion.uploads (status, expires_at)
    WHERE status = 'pending';

CREATE INDEX uploads_owner_id_idx ON ingestion.uploads (owner_id);

-- ----------------------------------------------------------------------
-- updated_at maintenance — single trigger reused across tables.
-- ----------------------------------------------------------------------
CREATE OR REPLACE FUNCTION library.set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER documents_set_updated_at
    BEFORE UPDATE ON library.documents
    FOR EACH ROW
    EXECUTE FUNCTION library.set_updated_at();

COMMIT;
