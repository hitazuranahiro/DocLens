package domain

import (
	"context"

	libdomain "github.com/tomeku/doclens/services/library/domain"
)

// Transactor runs the multi-write completion step atomically.
//
// The completion step writes (a) the artifact rows, (b) the
// document's ready status + metrics, and (c) the search index row
// (Property 5: extraction-success-or-nothing).
//
// The adapter (in apps/extraction-worker) opens one Postgres tx,
// rebinds the library and search repos to it, and invokes fn with
// the bound collaborators.
type Transactor interface {
	WithinReadyTx(ctx context.Context, fn func(library libdomain.Repository, indexer Indexer) error) error
}
