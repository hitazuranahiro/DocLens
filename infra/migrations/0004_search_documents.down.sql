-- 0004_search_documents.down.sql

BEGIN;

DROP TRIGGER IF EXISTS search_documents_set_updated_at ON search.documents;
DROP INDEX IF EXISTS search.documents_owner_id_idx;
DROP INDEX IF EXISTS search.documents_search_vector_gin_idx;
DROP TABLE IF EXISTS search.documents;

COMMIT;
