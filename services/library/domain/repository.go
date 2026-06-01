package domain

import (
	"context"

	"github.com/google/uuid"
)

// Repository is the persistence port for the Library context.
//
// Implementations live under adapters/postgres. Owner isolation
// (Property 2) is the responsibility of the implementation: every
// query MUST scope by owner_id.
type Repository interface {
	// FindAliveByOwnerSHA looks up an existing non-deleted document by
	// (ownerId, sha256). Returns ErrDocumentNotFound when absent.
	FindAliveByOwnerSHA(ctx context.Context, ownerID, sha256 string) (*Document, error)

	// Insert creates a new document row. Returns ErrDuplicateDocument if
	// the (ownerId, sha256) unique index is violated by a concurrent
	// insert.
	Insert(ctx context.Context, d *Document) error

	// FindByID returns a document scoped to the owner. Returns
	// ErrDocumentNotFound when absent OR when the row exists but
	// belongs to a different owner (per Req 7.9 — 404 not 403).
	FindByID(ctx context.Context, ownerID string, id uuid.UUID) (*Document, error)
}
