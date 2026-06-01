-- 0004_search_documents.up.sql
--
-- M7 — Postgres full-text search (per ADR 0010).
--
-- The Search context owns its own schema. We keep one row per ready
-- document with:
--
--   * owner_id          — for owner-scoped queries (Property 2).
--   * title             — denormalized so we can rank/headline against it.
--   * body              — extracted Markdown text the worker passes us.
--   * search_vector     — generated tsvector covering title (weight A)
--                         and body (weight B). 'english' config matches
--                         the Markdown produced by MarkItDown for our
--                         default corpus; multilingual is a v0.2 concern.
--
-- A single GIN index on `search_vector` covers `@@` queries; the
-- composite (`owner_id`, `search_vector`) form would be cute but
-- Postgres' planner already filters on owner_id with the partial
-- B-tree below.
--
-- Indexing is done in the same DB transaction as the extraction
-- worker's Mark-Ready step (Property 5). That transactional contract
-- lives in the Library Repository; this schema is just the storage.

BEGIN;

CREATE TABLE search.documents (
    document_id   uuid        PRIMARY KEY REFERENCES library.documents (id) ON DELETE CASCADE,
    owner_id      text        NOT NULL,
    title         text        NOT NULL,
    body          text        NOT NULL,
    search_vector tsvector    GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(body,  '')), 'B')
    ) STORED,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- Owner filter is selective enough that a partial GIN over
-- search_vector pays for itself; combining the two with a btree-gin
-- composite is overkill for v0.1 traffic.
CREATE INDEX documents_search_vector_gin_idx
    ON search.documents USING gin (search_vector);

-- Owner scoping for the WHERE filter applied before tsquery match.
CREATE INDEX documents_owner_id_idx
    ON search.documents (owner_id);

-- Reuse the shared updated_at trigger function from 0001.
CREATE TRIGGER search_documents_set_updated_at
    BEFORE UPDATE ON search.documents
    FOR EACH ROW
    EXECUTE FUNCTION library.set_updated_at();

COMMIT;
