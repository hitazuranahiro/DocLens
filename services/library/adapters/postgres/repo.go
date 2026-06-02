// Package postgres implements the Library Repository against Postgres
// using pgx/v5. We hand-write SQL today; sqlc is on the roadmap for when
// the query surface grows past a half-dozen statements.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/db"
)

// Repo persists Document aggregates.
type Repo struct {
	pool *pgxpool.Pool
	q    db.Querier
}

// New returns a Repo backed by the given pgx pool.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool, q: pool} }

// WithQuerier returns a copy of Repo bound to the given querier
// (typically a `pgx.Tx`). Used when the extraction worker needs
// MarkReady + UpsertArtifact + a search.Upsert in one transaction
// (Property 5).
//
// The returned Repo intentionally has a nil pool so any code path
// that accidentally tries to escape the transaction will fail loudly
// instead of silently running outside it.
func (r *Repo) WithQuerier(q db.Querier) *Repo {
	return &Repo{pool: nil, q: q}
}

// FindAliveByOwnerSHA implements domain.Repository.
func (r *Repo) FindAliveByOwnerSHA(ctx context.Context, ownerID, sha256 string) (*domain.Document, error) {
	const q = `
SELECT id, owner_id, title, source_filename, sha256, byte_size, mime_type,
       status, page_count, word_count, confidence, last_error,
       raw_object_key, created_at, updated_at
FROM library.documents
WHERE owner_id = $1 AND sha256 = $2 AND status <> 'deleted'`

	row := r.q.QueryRow(ctx, q, ownerID, sha256)
	d, err := scanDocument(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("library: find by owner/sha: %w", err)
	}
	return d, nil
}

// Insert implements domain.Repository.
func (r *Repo) Insert(ctx context.Context, d *domain.Document) error {
	const q = `
INSERT INTO library.documents (
    id, owner_id, title, source_filename, sha256, byte_size, mime_type,
    status, raw_object_key, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now(), now())`

	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	if d.Status == "" {
		d.Status = domain.StatusQueued
	}

	_, err := r.q.Exec(ctx, q,
		d.ID, d.OwnerID, d.Title, d.SourceFilename, d.SHA256, d.ByteSize,
		d.MimeType, string(d.Status), d.RawObjectKey,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		// 23505 unique_violation: hit the (owner_id, sha256) live index.
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateDocument
		}
		return fmt.Errorf("library: insert: %w", err)
	}
	return nil
}

// FindByID implements domain.Repository. Owner-scoped to satisfy Req 7.9.
func (r *Repo) FindByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Document, error) {
	const q = `
SELECT id, owner_id, title, source_filename, sha256, byte_size, mime_type,
       status, page_count, word_count, confidence, last_error,
       raw_object_key, created_at, updated_at
FROM library.documents
WHERE id = $1 AND owner_id = $2 AND status <> 'deleted'`

	row := r.q.QueryRow(ctx, q, id, ownerID)
	d, err := scanDocument(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("library: find by id: %w", err)
	}
	return d, nil
}

// rowScanner narrows the contract so scanDocument works for both
// QueryRow and Query (multi-row) callers later.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanDocument(row rowScanner) (*domain.Document, error) {
	var (
		d         domain.Document
		statusStr string
	)
	err := row.Scan(
		&d.ID, &d.OwnerID, &d.Title, &d.SourceFilename, &d.SHA256, &d.ByteSize,
		&d.MimeType, &statusStr, &d.PageCount, &d.WordCount, &d.Confidence,
		&d.LastError, &d.RawObjectKey, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	d.Status = domain.Status(statusStr)
	return &d, nil
}


// FindByIDUnscoped implements domain.Repository. Workers use this
// because the documentId in the asynq payload is already trusted
// (it was minted by the API while the upload was authenticated).
func (r *Repo) FindByIDUnscoped(ctx context.Context, id uuid.UUID) (*domain.Document, error) {
	const q = `
SELECT id, owner_id, title, source_filename, sha256, byte_size, mime_type,
       status, page_count, word_count, confidence, last_error,
       raw_object_key, created_at, updated_at
FROM library.documents
WHERE id = $1`

	row := r.q.QueryRow(ctx, q, id)
	d, err := scanDocument(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrDocumentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("library: find by id (unscoped): %w", err)
	}
	return d, nil
}

// MarkExtracting implements domain.Repository.
func (r *Repo) MarkExtracting(ctx context.Context, id uuid.UUID) error {
	// Allow queued -> extracting and the idempotent extracting ->
	// extracting. Refuse to overwrite ready/failed/deleted.
	const q = `
UPDATE library.documents
   SET status = 'extracting'
 WHERE id = $1 AND status IN ('queued', 'extracting')`

	tag, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("library: mark extracting: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}
	return nil
}

// MarkReady implements domain.Repository.
func (r *Repo) MarkReady(ctx context.Context, id uuid.UUID, m domain.ReadyMetrics) error {
	const q = `
UPDATE library.documents
   SET status = 'ready',
       page_count = $2,
       word_count = $3,
       confidence = $4,
       last_error = NULL
 WHERE id = $1 AND status <> 'deleted'`

	tag, err := r.q.Exec(ctx, q,
		id, m.PageCount, m.WordCount, m.Confidence,
	)
	if err != nil {
		return fmt.Errorf("library: mark ready: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}
	return nil
}

// MarkFailed implements domain.Repository.
func (r *Repo) MarkFailed(ctx context.Context, id uuid.UUID, reason string) error {
	const q = `
UPDATE library.documents
   SET status = 'failed',
       last_error = $2
 WHERE id = $1 AND status <> 'deleted'`

	tag, err := r.q.Exec(ctx, q, id, truncate(reason, 4096))
	if err != nil {
		return fmt.Errorf("library: mark failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}
	return nil
}

// MarkRetry implements domain.Repository. Owner-scoped because the
// retry endpoint is user-driven.
func (r *Repo) MarkRetry(ctx context.Context, ownerID string, id uuid.UUID) error {
	const q = `
UPDATE library.documents
   SET status = 'queued',
       last_error = NULL
 WHERE id = $1 AND owner_id = $2 AND status = 'failed'`

	tag, err := r.q.Exec(ctx, q, id, ownerID)
	if err != nil {
		return fmt.Errorf("library: mark retry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}
	return nil
}

// UpsertArtifact implements domain.Repository.
func (r *Repo) UpsertArtifact(ctx context.Context, a *domain.Artifact) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	const q = `
INSERT INTO library.artifacts (id, document_id, kind, object_key, byte_size, created_at)
VALUES ($1, $2, $3, $4, $5, now())
ON CONFLICT (document_id, kind)
DO UPDATE SET object_key = EXCLUDED.object_key,
              byte_size  = EXCLUDED.byte_size`

	_, err := r.q.Exec(ctx, q,
		a.ID, a.DocumentID, string(a.Kind), a.ObjectKey, a.ByteSize,
	)
	if err != nil {
		return fmt.Errorf("library: upsert artifact: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}


// ListByOwner implements domain.Repository.
//
// Sort order is (created_at desc, id desc) and pagination uses the
// row-tuple comparator so we can resume past identical timestamps:
//
//	WHERE owner_id = $1
//	  AND status <> 'deleted'
//	  AND (created_at, id) < ($cursorCreated, $cursorID)
//	ORDER BY created_at DESC, id DESC
//	LIMIT $limit + 1
//
// We fetch limit+1 to detect whether more rows exist; the extra row
// is dropped before returning.
func (r *Repo) ListByOwner(ctx context.Context, ownerID string, limit int, cursor *domain.Cursor) ([]*domain.Document, *domain.Cursor, error) {
	if limit <= 0 {
		limit = 20
	}

	const baseQ = `
SELECT id, owner_id, title, source_filename, sha256, byte_size, mime_type,
       status, page_count, word_count, confidence, last_error,
       raw_object_key, created_at, updated_at
FROM library.documents
WHERE owner_id = $1 AND status <> 'deleted'`

	var (
		rows pgx.Rows
		err  error
	)
	if cursor == nil {
		q := baseQ + ` ORDER BY created_at DESC, id DESC LIMIT $2`
		rows, err = r.q.Query(ctx, q, ownerID, limit+1)
	} else {
		q := baseQ + ` AND (created_at, id) < ($2, $3) ORDER BY created_at DESC, id DESC LIMIT $4`
		rows, err = r.q.Query(ctx, q, ownerID, cursor.CreatedAt, cursor.ID, limit+1)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("library: list by owner: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Document, 0, limit+1)
	for rows.Next() {
		d, err := scanDocument(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("library: list scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("library: list iter: %w", err)
	}

	var next *domain.Cursor
	if len(out) > limit {
		last := out[limit-1]
		next = &domain.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		out = out[:limit]
	}
	return out, next, nil
}

// FindArtifacts implements domain.Repository.
func (r *Repo) FindArtifacts(ctx context.Context, documentID uuid.UUID) ([]*domain.Artifact, error) {
	const q = `
SELECT id, document_id, kind, object_key, byte_size, created_at
FROM library.artifacts
WHERE document_id = $1
ORDER BY kind`

	rows, err := r.q.Query(ctx, q, documentID)
	if err != nil {
		return nil, fmt.Errorf("library: list artifacts: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.Artifact, 0, 4)
	for rows.Next() {
		var (
			a       domain.Artifact
			kindStr string
		)
		if err := rows.Scan(&a.ID, &a.DocumentID, &kindStr, &a.ObjectKey, &a.ByteSize, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("library: artifact scan: %w", err)
		}
		a.Kind = domain.ArtifactKind(kindStr)
		out = append(out, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("library: artifact iter: %w", err)
	}
	return out, nil
}


// SoftDelete implements domain.Repository.
//
// We perform the read-then-write under a single statement when we
// can; here we need both the raw_object_key and the artifact rows
// in the result, so we issue two queries against `r.q`. Callers
// that want atomicity wrap us in a tx via WithQuerier.
//
// The artifact rows are deleted in this call because they're
// owned by the document and useless once it's gone. The S3 objects
// they point at are returned so the caller can hard-delete them
// asynchronously (S3 is not transactional).
func (r *Repo) SoftDelete(ctx context.Context, ownerID string, id uuid.UUID) (string, []string, error) {
	// Step 1: collect the raw + artifact keys we'll need to clean
	// up. We read these BEFORE flipping status so a concurrent
	// reader can't race us.
	const docQ = `
SELECT raw_object_key, status
FROM library.documents
WHERE id = $1 AND owner_id = $2`

	var rawKey, statusStr string
	if err := r.q.QueryRow(ctx, docQ, id, ownerID).Scan(&rawKey, &statusStr); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, domain.ErrDocumentNotFound
		}
		return "", nil, fmt.Errorf("library: soft-delete lookup: %w", err)
	}

	// Idempotent: already deleted, return what we know.
	if statusStr == string(domain.StatusDeleted) {
		return rawKey, nil, nil
	}

	const artQ = `SELECT object_key FROM library.artifacts WHERE document_id = $1`
	rows, err := r.q.Query(ctx, artQ, id)
	if err != nil {
		return "", nil, fmt.Errorf("library: soft-delete artifacts: %w", err)
	}
	artKeys := make([]string, 0, 4)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return "", nil, fmt.Errorf("library: soft-delete artifact scan: %w", err)
		}
		artKeys = append(artKeys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", nil, fmt.Errorf("library: soft-delete artifact iter: %w", err)
	}

	// Step 2: flip the status. Cascade on the artifacts FK removes
	// the rows; the unique-alive index already excludes deleted
	// rows so the user can re-upload immediately.
	const updQ = `
UPDATE library.documents
   SET status = 'deleted',
       last_error = NULL
 WHERE id = $1 AND owner_id = $2 AND status <> 'deleted'`

	tag, err := r.q.Exec(ctx, updQ, id, ownerID)
	if err != nil {
		return "", nil, fmt.Errorf("library: soft-delete update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Race: deleted between our SELECT and UPDATE. Treat as
		// idempotent success.
		return rawKey, artKeys, nil
	}

	// Step 3: drop artifact rows. The S3 objects are still up;
	// the caller hard-deletes them asynchronously.
	const delArtQ = `DELETE FROM library.artifacts WHERE document_id = $1`
	if _, err := r.q.Exec(ctx, delArtQ, id); err != nil {
		return "", nil, fmt.Errorf("library: soft-delete clear artifacts: %w", err)
	}

	return rawKey, artKeys, nil
}

// HardDelete implements domain.Repository.
//
// Removes the document row outright. Foreign-key cascades take care
// of any leftover artifact rows. Caller MUST have already cleaned
// up the underlying S3 objects.
func (r *Repo) HardDelete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM library.documents WHERE id = $1 AND status = 'deleted'`
	tag, err := r.q.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("library: hard-delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDocumentNotFound
	}
	return nil
}
