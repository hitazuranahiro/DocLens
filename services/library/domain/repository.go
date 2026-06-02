package domain

import (
	"context"
	"time"

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

	// ListByOwner returns up to `limit` documents owned by ownerID,
	// most recent first. Pagination uses keyset on (created_at, id);
	// pass nil cursor for the first page. nextCursor is non-nil iff
	// more rows likely exist after this page.
	ListByOwner(ctx context.Context, ownerID string, limit int, cursor *Cursor) ([]*Document, *Cursor, error)

	// FindArtifacts returns every Artifact for the given document.
	// Owner-scoping is enforced by the caller (look up the doc
	// owner-scoped first; if it returns 404 don't call this).
	FindArtifacts(ctx context.Context, documentID uuid.UUID) ([]*Artifact, error)

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

	// SoftDelete flips status to 'deleted' for an owner-scoped doc.
	// Idempotent: deleting an already-deleted document returns nil
	// (the row was already gone for the user). Owner-scoped to keep
	// 404 vs 403 semantics clean (Req 7.9).
	//
	// Returns the document's raw_object_key + the artifact rows so
	// the caller can hard-delete the underlying S3 objects after the
	// transaction commits. The artifact rows themselves are deleted
	// here; they're useless once the document is gone.
	SoftDelete(ctx context.Context, ownerID string, id uuid.UUID) (rawKey string, artifactKeys []string, err error)

	// HardDeleteFor removes the document row and all dependents.
	// Used by the storage-cleanup sweeper after S3 objects are gone.
	// Caller must verify status='deleted' before invoking this.
	HardDelete(ctx context.Context, id uuid.UUID) error

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


// Cursor is the keyset cursor for owner-scoped document listings.
// Sort is by (created_at desc, id desc); the next page selects rows
// strictly after the (CreatedAt, ID) pair on the previous page.
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}
