// Package domain — Indexer port for downstream search indexing.
//
// The extraction worker is the only producer of indexable content
// in v0.1, so it owns this port. The contract is intentionally
// narrow: index a (documentId, ownerId, title, body) tuple.
//
// The current adapter is the search context's postgres Repo; a
// future hybrid (FTS + vectors) adapter could implement the same
// port without touching the extraction service.
package domain

import (
	"context"

	"github.com/google/uuid"
)

// Indexer is the search-side write port from the extraction
// worker's perspective.
type Indexer interface {
	// Upsert indexes (or re-indexes) a document.
	// `body` is the extracted text (Markdown is fine — Postgres FTS
	// happily tokenizes Markdown punctuation).
	Upsert(ctx context.Context, doc IndexedDocument) error
}

// IndexedDocument is the data we hand to the indexer. We use a
// small struct here rather than dragging library.domain.Document
// across context boundaries.
type IndexedDocument struct {
	DocumentID uuid.UUID
	OwnerID    string
	Title      string
	Body       string
}
