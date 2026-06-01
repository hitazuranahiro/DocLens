// Package domain holds the Search bounded context's port.
//
// Per ADR 0010, v0.1 search runs on Postgres FTS. The Repository
// interface keeps the storage decision behind a port so a future
// pgvector-backed adapter (or an external service) can be swapped
// in without touching callers.
package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Document is the indexable shape of a single library document.
type Document struct {
	DocumentID uuid.UUID
	OwnerID    string
	Title      string
	Body       string
}

// Hit is one search result row.
type Hit struct {
	DocumentID uuid.UUID
	OwnerID    string
	Title      string
	// Snippet is HTML-ish output of `ts_headline` with <mark> tags
	// around matching terms. Callers MUST sanitize before rendering
	// in the browser; we do not promise the snippet is safe by itself.
	Snippet string
	// Rank is `ts_rank_cd` for the document. Higher = better match.
	// Used as the primary sort key.
	Rank float64
}

// Cursor is the keyset cursor for paginated search results.
//
// Sort key is (rank desc, document_id desc); the next page selects
// rows strictly after the (Rank, DocumentID) pair. We compare by
// floats here because Postgres returns ts_rank_cd as a float4 and
// strict inequality on identical ranks is broken by document_id.
type Cursor struct {
	Rank       float64
	DocumentID uuid.UUID
}

// Repository is the Search persistence port.
type Repository interface {
	// Upsert writes (or replaces) the indexed row for a document.
	// Calls participate in a transaction supplied by the caller via
	// WithTx — see library.Repository.WithTx for the contract.
	Upsert(ctx context.Context, d Document) error

	// Delete removes the indexed row. Idempotent.
	Delete(ctx context.Context, documentID uuid.UUID) error

	// Search runs a full-text query scoped to one owner. q is the
	// raw user input; the adapter is responsible for safe parsing
	// (websearch_to_tsquery in the Postgres adapter). limit is the
	// page size; cursor (nil for first page) is the resume token.
	//
	// Returns (hits, nextCursor). nextCursor is non-nil iff more
	// rows likely exist after this page.
	Search(ctx context.Context, ownerID, q string, limit int, cursor *Cursor) ([]Hit, *Cursor, error)
}

// ErrEmptyQuery is returned when the caller supplies a blank query.
// The handler maps this to HTTP 400.
var ErrEmptyQuery = errors.New("search: empty query")
