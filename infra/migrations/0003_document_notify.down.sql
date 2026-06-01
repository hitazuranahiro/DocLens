-- 0003_document_notify.down.sql

BEGIN;

DROP TRIGGER IF EXISTS documents_notify_update ON library.documents;
DROP TRIGGER IF EXISTS documents_notify_insert ON library.documents;
DROP FUNCTION IF EXISTS library.notify_document_status();

COMMIT;
