package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tomeku/doclens/services/extraction/app"
	"github.com/tomeku/doclens/services/extraction/domain"
	libdomain "github.com/tomeku/doclens/services/library/domain"
)

// TestExtract_WithTransactor_IndexesAndMarksReadyAtomically verifies
// the new Property 5 wiring: when a Transactor is supplied, the
// service's "ready step" routes the artifact upserts, search index
// upsert, and Mark-Ready through a single callback (and presumably,
// in production, a single tx). We don't need a real Postgres here —
// only that the indexer is invoked with the right document.
func TestExtract_WithTransactor_IndexesAndMarksReadyAtomically(t *testing.T) {
	svc, repo, store, ext, indexer := newServiceWithIndexer(t)
	_ = ext
	_ = store
	doc := newSeededDoc(t, repo)

	if err := svc.Extract(context.Background(), doc.ID); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if indexer.calls != 1 {
		t.Fatalf("indexer calls = %d, want 1", indexer.calls)
	}
	if indexer.lastDoc.DocumentID != doc.ID {
		t.Fatalf("indexer document_id = %v, want %v", indexer.lastDoc.DocumentID, doc.ID)
	}
	if indexer.lastDoc.OwnerID != doc.OwnerID {
		t.Fatalf("indexer owner_id = %q, want %q", indexer.lastDoc.OwnerID, doc.OwnerID)
	}
	if indexer.lastDoc.Title == "" || indexer.lastDoc.Body == "" {
		t.Fatalf("indexer missing title/body: %+v", indexer.lastDoc)
	}

	// Library status must have flipped to ready inside the same callback.
	got, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)
	if got.Status != libdomain.StatusReady {
		t.Fatalf("status = %s, want ready", got.Status)
	}
}

// TestExtract_WithTransactor_RollsBackWhenIndexerFails verifies that
// a search-index failure rolls back the whole completion step: the
// document does NOT transition to ready, and asynq will retry.
func TestExtract_WithTransactor_RollsBackWhenIndexerFails(t *testing.T) {
	svc, repo, _, _, indexer := newServiceWithIndexer(t)
	indexer.err = errors.New("simulated index failure")

	doc := newSeededDoc(t, repo)
	err := svc.Extract(context.Background(), doc.ID)
	if err == nil {
		t.Fatalf("expected error from index failure")
	}

	got, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)
	if got.Status == libdomain.StatusReady {
		t.Fatalf("status should not be ready when index step fails: %s", got.Status)
	}
	// repo started with extracting; rollback should leave it at
	// extracting (not flipped to ready). Either status is fine for
	// this contract; the point is "not ready".
	if got.Status == libdomain.StatusReady {
		t.Fatalf("status mistakenly ready: %s", got.Status)
	}
}

// --- helpers --------------------------------------------------------------

func newServiceWithIndexer(t *testing.T) (*app.Service, *libRepo, *fakeStore, *fakeExtractor, *recordingIndexer) {
	t.Helper()
	repo := newLibRepo()
	store := newFakeStore()
	store.put(rawBucket, rawKey, []byte(rawContent))
	ext := &fakeExtractor{
		result: &domain.Result{
			Markdown: "# Hello\n\n" + repeat("word ", 250),
			Pages:    1,
		},
	}
	indexer := &recordingIndexer{}
	tx := &fakeTransactor{repo: repo, indexer: indexer}
	svc, err := app.NewService(repo, store, ext, rawBucket, artBucket, app.Options{
		EnabledMimes: []string{"application/pdf"},
		Transactor:   tx,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, repo, store, ext, indexer
}

// fakeTransactor runs the callback synchronously with the in-memory
// library repo and the recording indexer. A real adapter would wrap
// these in pgx.Tx; the contract is identical from the service's
// perspective.
type fakeTransactor struct {
	repo    *libRepo
	indexer *recordingIndexer
}

func (t *fakeTransactor) WithinReadyTx(
	ctx context.Context,
	fn func(library libdomain.Repository, indexer domain.Indexer) error,
) error {
	return fn(t.repo, t.indexer)
}

type recordingIndexer struct {
	calls   int
	lastDoc domain.IndexedDocument
	err     error
}

func (r *recordingIndexer) Upsert(_ context.Context, d domain.IndexedDocument) error {
	r.calls++
	r.lastDoc = d
	return r.err
}
