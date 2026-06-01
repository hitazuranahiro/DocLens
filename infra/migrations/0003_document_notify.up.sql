-- 0003_document_notify.up.sql
--
-- M6 — Live status (SSE).
--
-- The web client opens an EventSource against /v1/documents/stream. The
-- API listens on the Postgres channel `document_status` (LISTEN/NOTIFY)
-- and fans payloads out to subscribers scoped by owner_id.
--
-- We fire on INSERT and on UPDATE of any column the client cares about
-- (status changes are the headline; metrics + last_error matter when
-- the doc transitions to ready/failed). Per ADR 0009, this channel is
-- best-effort: durability lives in asynq and Postgres rows.
--
-- Payload size cap: pg_notify enforces 8000 bytes by default. We send a
-- compact JSON document; even with a long error tail (truncated to
-- 1024 chars) we stay well under the limit.

BEGIN;

CREATE OR REPLACE FUNCTION library.notify_document_status()
RETURNS trigger AS $$
DECLARE
    payload jsonb;
    err_text text;
BEGIN
    err_text := COALESCE(NEW.last_error, '');
    IF length(err_text) > 1024 THEN
        err_text := substr(err_text, 1, 1024);
    END IF;

    payload := jsonb_build_object(
        'event',        TG_OP,
        'owner_id',     NEW.owner_id,
        'document_id',  NEW.id::text,
        'status',       NEW.status::text,
        'page_count',   NEW.page_count,
        'word_count',   NEW.word_count,
        'confidence',   NEW.confidence,
        'last_error',   NULLIF(err_text, ''),
        'updated_at',   to_char(NEW.updated_at AT TIME ZONE 'UTC',
                                'YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
    );

    -- pg_notify silently truncates payloads above 8000 bytes; cap our
    -- last_error well below that to be safe.
    PERFORM pg_notify('document_status', payload::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- INSERT: a freshly finalized document should appear in any open list.
CREATE TRIGGER documents_notify_insert
    AFTER INSERT ON library.documents
    FOR EACH ROW
    EXECUTE FUNCTION library.notify_document_status();

-- UPDATE: only fire when something the client cares about changed.
-- Notably, raw_object_key updates and identical-status writes do not
-- spam the channel.
CREATE TRIGGER documents_notify_update
    AFTER UPDATE ON library.documents
    FOR EACH ROW
    WHEN (
        OLD.status      IS DISTINCT FROM NEW.status
     OR OLD.page_count  IS DISTINCT FROM NEW.page_count
     OR OLD.word_count  IS DISTINCT FROM NEW.word_count
     OR OLD.confidence  IS DISTINCT FROM NEW.confidence
     OR OLD.last_error  IS DISTINCT FROM NEW.last_error
    )
    EXECUTE FUNCTION library.notify_document_status();

COMMIT;
