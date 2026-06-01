package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository is the persistence port for the Ingestion context.
type Repository interface {
	// Insert creates a pending upload row. The caller must have
	// generated the object key and bucket before calling.
	Insert(ctx context.Context, u *Upload) error

	// FindByID returns the pending upload, scoped to the owner.
	// Returns ErrUploadNotFound when absent or owned by someone else.
	FindByID(ctx context.Context, ownerID string, id uuid.UUID) (*Upload, error)

	// MarkFinalized records that the upload has produced a Document.
	MarkFinalized(ctx context.Context, id, documentID uuid.UUID, at time.Time) error

	// ListExpired returns pending uploads whose expires_at is before
	// the given cutoff. Used by the orphan-sweep cron.
	ListExpired(ctx context.Context, before time.Time, limit int) ([]*Upload, error)

	// MarkExpired flips status to 'expired' for the given IDs.
	MarkExpired(ctx context.Context, ids []uuid.UUID) error
}
