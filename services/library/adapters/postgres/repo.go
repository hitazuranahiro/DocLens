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
)

// Repo persists Document aggregates.
type Repo struct {
	pool *pgxpool.Pool
}

// New returns a Repo backed by the given pgx pool.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// FindAliveByOwnerSHA implements domain.Repository.
func (r *Repo) FindAliveByOwnerSHA(ctx context.Context, ownerID, sha256 string) (*domain.Document, error) {
	const q = `
SELECT id, owner_id, title, source_filename, sha256, byte_size, mime_type,
       status, page_count, word_count, confidence, last_error,
       raw_object_key, created_at, updated_at
FROM library.documents
WHERE owner_id = $1 AND sha256 = $2 AND status <> 'deleted'`

	row := r.pool.QueryRow(ctx, q, ownerID, sha256)
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

	_, err := r.pool.Exec(ctx, q,
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

	row := r.pool.QueryRow(ctx, q, id, ownerID)
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

	row := r.pool.QueryRow(ctx, q, id)
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

	tag, err := r.pool.Exec(ctx, q, id)
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

	tag, err := r.pool.Exec(ctx, q,
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

	tag, err := r.pool.Exec(ctx, q, id, truncate(reason, 4096))
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

	tag, err := r.pool.Exec(ctx, q, id, ownerID)
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

	_, err := r.pool.Exec(ctx, q,
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
