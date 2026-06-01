-- 0002_extraction_jobs.down.sql
-- Local-development rollback only.

BEGIN;

DROP TRIGGER IF EXISTS extraction_jobs_set_updated_at ON extraction.jobs;
DROP INDEX IF EXISTS extraction.jobs_status_idx;
DROP INDEX IF EXISTS extraction.jobs_document_id_created_at_idx;
DROP TABLE IF EXISTS extraction.jobs;
DROP TYPE IF EXISTS extraction.job_status;

COMMIT;
