-- 0001_init.down.sql
--
-- Local-development rollback only. CI runs migrations forward-only.

BEGIN;

DROP TRIGGER IF EXISTS documents_set_updated_at ON library.documents;
DROP FUNCTION IF EXISTS library.set_updated_at();

DROP INDEX IF EXISTS ingestion.uploads_owner_id_idx;
DROP INDEX IF EXISTS ingestion.uploads_status_expires_at_idx;
DROP TABLE IF EXISTS ingestion.uploads;
DROP TYPE IF EXISTS ingestion.upload_status;

DROP INDEX IF EXISTS library.artifacts_document_id_idx;
DROP TABLE IF EXISTS library.artifacts;
DROP TYPE IF EXISTS library.artifact_kind;

DROP INDEX IF EXISTS library.documents_owner_created_at_idx;
DROP INDEX IF EXISTS library.documents_owner_sha256_alive_uk;
DROP TABLE IF EXISTS library.documents;
DROP TYPE IF EXISTS library.document_status;

DROP SCHEMA IF EXISTS search;
DROP SCHEMA IF EXISTS extraction;
DROP SCHEMA IF EXISTS library;
DROP SCHEMA IF EXISTS ingestion;

COMMIT;
