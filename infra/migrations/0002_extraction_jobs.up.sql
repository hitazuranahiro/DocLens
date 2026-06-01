-- 0002_extraction_jobs.up.sql
--
-- M4 schema for the Extraction context. Each row mirrors one asynq
-- task so the API and a future ops UI can see job state without
-- talking to Redis. Asynq itself remains the source of truth for
-- queue/retry mechanics; this table is a denormalized projection.

BEGIN;

CREATE TYPE extraction.job_status AS ENUM (
    'queued',     -- enqueued, no worker has picked it up yet
    'running',    -- a worker is processing this attempt
    'succeeded',  -- terminal: produced artifacts, document ready
    'failed'      -- terminal: gave up after retries
);

CREATE TABLE extraction.jobs (
    id            uuid                   PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id   uuid                   NOT NULL REFERENCES library.documents (id) ON DELETE CASCADE,
    status        extraction.job_status  NOT NULL DEFAULT 'queued',
    attempts      integer                NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error    text                   NULL,
    -- Asynq's task ID for this job. Used to correlate logs/metrics
    -- across the API enqueue path and the worker.
    task_id       text                   NULL,
    started_at    timestamptz            NULL,
    completed_at  timestamptz            NULL,
    created_at    timestamptz            NOT NULL DEFAULT now(),
    updated_at    timestamptz            NOT NULL DEFAULT now()
);

-- Look up the most recent job for a document (the retry endpoint and
-- the SSE feed both want this).
CREATE INDEX jobs_document_id_created_at_idx
    ON extraction.jobs (document_id, created_at DESC);

-- Find runnable / running jobs at a glance for ops dashboards.
CREATE INDEX jobs_status_idx ON extraction.jobs (status);

-- Reuse the updated_at trigger function defined by 0001 in the
-- library schema. It's just plpgsql with no schema-specific binding.
CREATE TRIGGER extraction_jobs_set_updated_at
    BEFORE UPDATE ON extraction.jobs
    FOR EACH ROW
    EXECUTE FUNCTION library.set_updated_at();

COMMIT;
