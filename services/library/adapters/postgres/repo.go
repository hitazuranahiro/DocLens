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
