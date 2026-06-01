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

	// FindByIDUnscoped returns a document by ID without an owner
	// filter. Workers (which dispatch on a documentId carried in a
	// trusted job payload) need this; HTTP callers must not.
	FindByIDUnscoped(ctx context.Context, id uuid.UUID) (*Document, error)

	// MarkExtracting flips status to 'extracting' and records the
	// attempt start. Idempotent if the row is already 'extracting'.
	// Returns ErrInvalidTransition when the row is in a terminal
	// state we should not overwrite (e.g. 'deleted').
	MarkExtracting(ctx context.Context, id uuid.UUID) error

	// MarkReady writes the post-extraction metrics and flips status
	// to 'ready' atomically. last_error is cleared. Idempotent.
	MarkReady(ctx context.Context, id uuid.UUID, m ReadyMetrics) error

	// MarkFailed records the failure reason and flips status to 'failed'.
	// Idempotent.
	MarkFailed(ctx context.Context, id uuid.UUID, reason string) error

	// MarkRetry transitions a 'failed' document back to 'queued' so a
	// fresh attempt can run. Returns ErrInvalidTransition for any
	// other source state.
	MarkRetry(ctx context.Context, ownerID string, id uuid.UUID) error

	// UpsertArtifact replaces or inserts an artifact for the
	// (documentId, kind) tuple. Atomic; matches the unique constraint
	// in the schema.
	UpsertArtifact(ctx context.Context, a *Artifact) error
}

// ReadyMetrics is the bundle of post-extraction values we persist on
// the document row.
type ReadyMetrics struct {
	PageCount  int
	WordCount  int
	Confidence int // 0..100
}
