package readytx

import (
	"context"

	extractdomain "github.com/tomeku/doclens/services/extraction/domain"
	searchdomain "github.com/tomeku/doclens/services/search/domain"
	searchpg "github.com/tomeku/doclens/services/search/adapters/postgres"
)

// indexerAdapter exposes the search context's postgres Repo as the
// extraction context's narrow Indexer port. Translating between two
// domain types here keeps either context free to evolve its
// internal model without breaking the other.
type indexerAdapter struct {
	repo *searchpg.Repo
}

// Upsert implements extractdomain.Indexer.
func (a *indexerAdapter) Upsert(ctx context.Context, d extractdomain.IndexedDocument) error {
	return a.repo.Upsert(ctx, searchdomain.Document{
		DocumentID: d.DocumentID,
		OwnerID:    d.OwnerID,
		Title:      d.Title,
		Body:       d.Body,
	})
}
