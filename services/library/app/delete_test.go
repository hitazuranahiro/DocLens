package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/library/app"
	"github.com/tomeku/doclens/services/library/domain"
)

func TestDelete_HappyPath_NoTransactor(t *testing.T) {
	svc, repo, _ := newSvc(t)
	doc := seedReadyDoc(t, repo)
	eraser := &fakeEraser{}
	store := &fakeDeleteStore{}
	svc.SetDeleteDeps(nil, eraser, store, nil)

	res, err := svc.Delete(context.Background(), doc.OwnerID, doc.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if res.AlreadyDeleted {
		t.Fatalf("AlreadyDeleted = true; expected first-time delete")
	}

	got := repo.docs[doc.ID]
	if got.Status != domain.StatusDeleted {
		t.Fatalf("status = %s, want deleted", got.Status)
	}
	if eraser.calls != 1 {
		t.Fatalf("eraser calls = %d, want 1", eraser.calls)
	}

	// S3 cleanup is async; wait briefly then check.
	if !waitForCount(func() int { return store.deletes() }, 3, time.Second) {
		t.Fatalf("expected 3 S3 deletes (raw + 2 artifacts), got %d", store.deletes())
	}
}

func TestDelete_NotFound_404Like(t *testing.T) {
	svc, _, _ := newSvc(t)
	svc.SetDeleteDeps(nil, &fakeEraser{}, &fakeDeleteStore{}, nil)

	_, err := svc.Delete(context.Background(), "user_42", uuid.New())
	if !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("got err=%v, want ErrDocumentNotFound", err)
	}
}

func TestDelete_OwnerScoped(t *testing.T) {
	svc, repo, _ := newSvc(t)
	doc := seedReadyDoc(t, repo)
	svc.SetDeleteDeps(nil, &fakeEraser{}, &fakeDeleteStore{}, nil)

	// Different user attempting delete → 404 (Req 7.9).
	_, err := svc.Delete(context.Background(), "user_99", doc.ID)
	if !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("cross-owner delete should 404, got err=%v", err)
	}
	// Original doc still alive.
	if repo.docs[doc.ID].Status == domain.StatusDeleted {
		t.Fatalf("cross-owner delete leaked through")
	}
}

func TestDelete_Idempotent(t *testing.T) {
	svc, repo, _ := newSvc(t)
	doc := seedReadyDoc(t, repo)
	svc.SetDeleteDeps(nil, &fakeEraser{}, &fakeDeleteStore{}, nil)

	if _, err := svc.Delete(context.Background(), doc.OwnerID, doc.ID); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	res, err := svc.Delete(context.Background(), doc.OwnerID, doc.ID)
	if err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if !res.AlreadyDeleted {
		t.Fatalf("second call should report AlreadyDeleted")
	}
}

// TestDelete_WithTransactor_RollsBackOnEraserFailure verifies that
// a tx callback that returns an error prevents the status flip.
//
// The fake tx mirrors a real Postgres tx: it snapshots the repo
// state on entry, runs fn, and reverts the snapshot if fn errors.
// When the real adapter wraps Library.SoftDelete + IndexEraser.Delete
// in one pgx.Tx, Postgres provides this rollback for free; here we
// simulate it with map snapshots.
func TestDelete_WithTransactor_RollsBackOnEraserFailure(t *testing.T) {
	svc, repo, _ := newSvc(t)
	doc := seedReadyDoc(t, repo)
	eraser := &fakeEraser{err: errors.New("simulated index failure")}
	store := &fakeDeleteStore{}
	tx := &fakeDeleteTx{repo: repo, eraser: eraser}
	svc.SetDeleteDeps(tx, eraser, store, nil)

	_, err := svc.Delete(context.Background(), doc.OwnerID, doc.ID)
	if err == nil {
		t.Fatalf("expected error from eraser failure")
	}
	got := repo.docs[doc.ID]
	if got.Status == domain.StatusDeleted {
		t.Fatalf("status flipped to deleted despite eraser failure: %s", got.Status)
	}
}

// --- helpers --------------------------------------------------------------

func seedReadyDoc(t *testing.T, r *fakeRepo) *domain.Document {
	t.Helper()
	id := uuid.New()
	d := &domain.Document{
		ID:           id,
		OwnerID:      "user_42",
		Title:        "Sample.pdf",
		Status:       domain.StatusReady,
		RawObjectKey: "raw/user_42/" + id.String() + "/file.pdf",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	r.docs[id] = d
	r.arts[id] = []*domain.Artifact{
		{DocumentID: id, Kind: domain.ArtifactMarkdown, ObjectKey: "artifacts/" + id.String() + "/extracted.md"},
		{DocumentID: id, Kind: domain.ArtifactThumbnail, ObjectKey: "artifacts/" + id.String() + "/thumbnail.png"},
	}
	return d
}

func waitForCount(get func() int, want int, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if get() >= want {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return get() >= want
}

type fakeEraser struct {
	calls int
	err   error
}

func (f *fakeEraser) Delete(_ context.Context, _ uuid.UUID) error {
	f.calls++
	return f.err
}

type fakeDeleteStore struct {
	mu  sync.Mutex
	got []string
}

func (f *fakeDeleteStore) Delete(_ context.Context, bucket, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, bucket+"/"+key)
	return nil
}

func (f *fakeDeleteStore) deletes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.got)
}

// fakeDeleteTx simulates a Postgres tx around library + eraser writes.
// Snapshots state on entry; reverts on error.
type fakeDeleteTx struct {
	repo   *fakeRepo
	eraser app.IndexEraser
}

func (t *fakeDeleteTx) WithinDeleteTx(
	ctx context.Context,
	fn func(library domain.Repository, eraser app.IndexEraser) error,
) error {
	preDocs := snapshotDocs(t.repo)
	preArts := snapshotArts(t.repo)

	if err := fn(t.repo, t.eraser); err != nil {
		t.repo.docs = preDocs
		t.repo.arts = preArts
		return err
	}
	return nil
}

func snapshotDocs(r *fakeRepo) map[uuid.UUID]*domain.Document {
	out := make(map[uuid.UUID]*domain.Document, len(r.docs))
	for k, v := range r.docs {
		dup := *v
		out[k] = &dup
	}
	return out
}

func snapshotArts(r *fakeRepo) map[uuid.UUID][]*domain.Artifact {
	out := make(map[uuid.UUID][]*domain.Artifact, len(r.arts))
	for k, v := range r.arts {
		c := make([]*domain.Artifact, len(v))
		copy(c, v)
		out[k] = c
	}
	return out
}
