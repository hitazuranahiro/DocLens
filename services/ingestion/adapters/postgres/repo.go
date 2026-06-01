// Package postgres implements ingestion.Repository over pgx/v5.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tomeku/doclens/services/ingestion/domain"
)

// Repo persists Upload rows.
type Repo struct {
	pool *pgxpool.Pool
}

// New returns a Repo backed by the given pool.
func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// Insert implements domain.Repository.
func (r *Repo) Insert(ctx context.Context, u *domain.Upload) error {
	const q = `
INSERT INTO ingestion.uploads (
    id, owner_id, object_key, bucket, sha256, mime_type, byte_size,
    source_filename, title, status, expires_at, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now())`

	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.Status == "" {
		u.Status = domain.UploadStatusPending
	}

	_, err := r.pool.Exec(ctx, q,
		u.ID, u.OwnerID, u.ObjectKey, u.Bucket, u.SHA256, u.MimeType, u.ByteSize,
		u.SourceFilename, u.Title, string(u.Status), u.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("ingestion: insert upload: %w", err)
	}
	return nil
}

// FindByID implements domain.Repository.
func (r *Repo) FindByID(ctx context.Context, ownerID string, id uuid.UUID) (*domain.Upload, error) {
	const q = `
SELECT id, owner_id, document_id, object_key, bucket, sha256, mime_type,
       byte_size, source_filename, title, status, expires_at, created_at,
       finalized_at
FROM ingestion.uploads
WHERE id = $1 AND owner_id = $2`

	row := r.pool.QueryRow(ctx, q, id, ownerID)
	u, err := scanUpload(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUploadNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("ingestion: find upload: %w", err)
	}
	return u, nil
}

// MarkFinalized implements domain.Repository.
func (r *Repo) MarkFinalized(ctx context.Context, id, documentID uuid.UUID, at time.Time) error {
	const q = `
UPDATE ingestion.uploads
   SET status = 'finalized', document_id = $2, finalized_at = $3
 WHERE id = $1 AND status = 'pending'`

	tag, err := r.pool.Exec(ctx, q, id, documentID, at)
	if err != nil {
		return fmt.Errorf("ingestion: mark finalized: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// The row was already finalized (concurrent /finalize) or
		// expired. Treat as not-found rather than silently succeeding.
		return domain.ErrUploadNotFound
	}
	return nil
}

// ListExpired implements domain.Repository.
func (r *Repo) ListExpired(ctx context.Context, before time.Time, limit int) ([]*domain.Upload, error) {
	const q = `
SELECT id, owner_id, document_id, object_key, bucket, sha256, mime_type,
       byte_size, source_filename, title, status, expires_at, created_at,
       finalized_at
FROM ingestion.uploads
WHERE status = 'pending' AND expires_at < $1
ORDER BY expires_at
LIMIT $2`

	rows, err := r.pool.Query(ctx, q, before, limit)
	if err != nil {
		return nil, fmt.Errorf("ingestion: list expired: %w", err)
	}
	defer rows.Close()

	var out []*domain.Upload
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, fmt.Errorf("ingestion: scan expired: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ingestion: list expired iter: %w", err)
	}
	return out, nil
}

// MarkExpired implements domain.Repository.
func (r *Repo) MarkExpired(ctx context.Context, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `UPDATE ingestion.uploads SET status = 'expired' WHERE id = ANY($1) AND status = 'pending'`
	_, err := r.pool.Exec(ctx, q, ids)
	if err != nil {
		return fmt.Errorf("ingestion: mark expired: %w", err)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUpload(row rowScanner) (*domain.Upload, error) {
	var (
		u         domain.Upload
		statusStr string
	)
	err := row.Scan(
		&u.ID, &u.OwnerID, &u.DocumentID, &u.ObjectKey, &u.Bucket, &u.SHA256,
		&u.MimeType, &u.ByteSize, &u.SourceFilename, &u.Title, &statusStr,
		&u.ExpiresAt, &u.CreatedAt, &u.FinalizedAt,
	)
	if err != nil {
		return nil, err
	}
	u.Status = domain.UploadStatus(statusStr)
	return &u, nil
}
