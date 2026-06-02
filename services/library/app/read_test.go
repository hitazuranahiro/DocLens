package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/library/app"
	"github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/storage"
)

// --- fakes ----------------------------------------------------------------

type fakeRepo struct {
	docs map[uuid.UUID]*domain.Document
	arts map[uuid.UUID][]*domain.Artifact
}

func newRepo() *fakeRepo {
	return &fakeRepo{
		docs: map[uuid.UUID]*domain.Document{},
		arts: map[uuid.UUID][]*domain.Artifact{},
	}
}
func (r *fakeRepo) FindAliveByOwnerSHA(context.Context, string, string) (*domain.Document, error) {
	return nil, errors.New("not used")
}
func (r *fakeRepo) Insert(context.Context, *domain.Document) error { return errors.New("not used") }
func (r *fakeRepo) FindByID(_ context.Context, ownerID string, id uuid.UUID) (*domain.Document, error) {
	d, ok := r.docs[id]
	if !ok || d.OwnerID != ownerID || d.Status == domain.StatusDeleted {
		return nil, domain.ErrDocumentNotFound
	}
	return d, nil
}
func (r *fakeRepo) FindByIDUnscoped(_ context.Context, id uuid.UUID) (*domain.Document, error) {
	d, ok := r.docs[id]
	if !ok {
		return nil, domain.ErrDocumentNotFound
	}
	return d, nil
}
func (r *fakeRepo) ListByOwner(_ context.Context, ownerID string, limit int, cursor *domain.Cursor) ([]*domain.Document, *domain.Cursor, error) {
	// Collect, sort by (CreatedAt desc, ID desc), filter, and slice.
	owned := make([]*domain.Document, 0)
	for _, d := range r.docs {
		if d.OwnerID == ownerID && d.Status != domain.StatusDeleted {
			owned = append(owned, d)
		}
	}
	sort.Slice(owned, func(i, j int) bool {
		if !owned[i].CreatedAt.Equal(owned[j].CreatedAt) {
			return owned[i].CreatedAt.After(owned[j].CreatedAt)
		}
		return owned[i].ID.String() > owned[j].ID.String()
	})
	if cursor != nil {
		out := owned[:0]
		for _, d := range owned {
			// (created, id) < (cursor.CreatedAt, cursor.ID)
			if d.CreatedAt.Before(cursor.CreatedAt) ||
				(d.CreatedAt.Equal(cursor.CreatedAt) && d.ID.String() < cursor.ID.String()) {
				out = append(out, d)
			}
		}
		owned = out
	}
	var next *domain.Cursor
	if len(owned) > limit {
		last := owned[limit-1]
		next = &domain.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		owned = owned[:limit]
	}
	return owned, next, nil
}
func (r *fakeRepo) FindArtifacts(_ context.Context, id uuid.UUID) ([]*domain.Artifact, error) {
	return r.arts[id], nil
}
func (r *fakeRepo) MarkExtracting(context.Context, uuid.UUID) error  { return nil }
func (r *fakeRepo) MarkReady(context.Context, uuid.UUID, domain.ReadyMetrics) error {
	return nil
}
func (r *fakeRepo) MarkFailed(context.Context, uuid.UUID, string) error { return nil }
func (r *fakeRepo) MarkRetry(context.Context, string, uuid.UUID) error  { return nil }
func (r *fakeRepo) UpsertArtifact(context.Context, *domain.Artifact) error {
	return nil
}
func (r *fakeRepo) SoftDelete(_ context.Context, ownerID string, id uuid.UUID) (string, []string, error) {
	d, ok := r.docs[id]
	if !ok || d.OwnerID != ownerID {
		return "", nil, domain.ErrDocumentNotFound
	}
	if d.Status == domain.StatusDeleted {
		return d.RawObjectKey, nil, nil
	}
	keys := make([]string, 0, len(r.arts[id]))
	for _, a := range r.arts[id] {
		keys = append(keys, a.ObjectKey)
	}
	d.Status = domain.StatusDeleted
	delete(r.arts, id)
	return d.RawObjectKey, keys, nil
}
func (r *fakeRepo) HardDelete(_ context.Context, id uuid.UUID) error {
	d, ok := r.docs[id]
	if !ok || d.Status != domain.StatusDeleted {
		return domain.ErrDocumentNotFound
	}
	delete(r.docs, id)
	return nil
}

type fakeStore struct {
	objects map[string][]byte // bucket+"/"+key
	getErr  error
}

func newStore() *fakeStore { return &fakeStore{objects: map[string][]byte{}} }

func (f *fakeStore) PresignPut(context.Context, string, string, storage.PresignPutOptions) (storage.PresignedURL, error) {
	return storage.PresignedURL{}, errors.New("not used")
}
func (f *fakeStore) PresignGet(_ context.Context, bucket, key string, ttl time.Duration) (storage.PresignedURL, error) {
	return storage.PresignedURL{
		URL:       "https://signed.example/" + bucket + "/" + key,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}
func (f *fakeStore) Head(context.Context, string, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, errors.New("not used")
}
func (f *fakeStore) Get(_ context.Context, bucket, key string) (io.ReadCloser, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	b, ok := f.objects[bucket+"/"+key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (f *fakeStore) Put(context.Context, string, string, io.Reader, storage.PutOptions) error {
	return errors.New("not used")
}
func (f *fakeStore) Delete(context.Context, string, string) error { return nil }

// --- helpers --------------------------------------------------------------

const (
	owner = "user_42"
	other = "user_99"
)

func newSvc(t *testing.T) (*app.Service, *fakeRepo, *fakeStore) {
	t.Helper()
	repo := newRepo()
	store := newStore()
	svc, err := app.NewService(repo, store, "doclens-raw", "doclens-artifacts")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, repo, store
}

func seedReady(t *testing.T, repo *fakeRepo, store *fakeStore, ownerID string, createdAt time.Time, mdBody string) *domain.Document {
	t.Helper()
	id := uuid.New()
	doc := &domain.Document{
		ID:             id,
		OwnerID:        ownerID,
		Title:          "Doc " + id.String()[:6],
		SourceFilename: "doc.pdf",
		SHA256:         "0000000000000000000000000000000000000000000000000000000000000001",
		ByteSize:       int64(len(mdBody)),
		MimeType:       "application/pdf",
		Status:         domain.StatusReady,
		RawObjectKey:   "raw/" + ownerID + "/" + id.String() + "/doc.pdf",
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	repo.docs[id] = doc

	mdKey := "artifacts/" + id.String() + "/extracted.md"
	repo.arts[id] = []*domain.Artifact{
		{
			ID: uuid.New(), DocumentID: id, Kind: domain.ArtifactMarkdown,
			ObjectKey: mdKey, ByteSize: int64(len(mdBody)),
		},
	}
	store.objects["doclens-artifacts/"+mdKey] = []byte(mdBody)
	return doc
}

// --- tests ----------------------------------------------------------------

func TestList_FirstPageReturnsRecentFirst(t *testing.T) {
	svc, repo, store := newSvc(t)
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	a := seedReady(t, repo, store, owner, now.Add(-1*time.Hour), "older")
	b := seedReady(t, repo, store, owner, now, "newer")

	page, err := svc.List(context.Background(), owner, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(page.Items))
	}
	if page.Items[0].ID != b.ID {
		t.Fatalf("first item should be the newer doc; got %v want %v", page.Items[0].ID, b.ID)
	}
	if page.Items[1].ID != a.ID {
		t.Fatalf("second item ID = %v, want %v", page.Items[1].ID, a.ID)
	}
	if page.NextCursor != "" {
		t.Fatalf("nextCursor should be empty when fewer than 20 items: %q", page.NextCursor)
	}
}

func TestList_PaginatesPastDefaultPageSize(t *testing.T) {
	svc, repo, store := newSvc(t)
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	// Seed 25 docs; the page size is 20.
	for i := 0; i < 25; i++ {
		seedReady(t, repo, store, owner, base.Add(time.Duration(i)*time.Minute), "x")
	}

	first, err := svc.List(context.Background(), owner, "")
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(first.Items) != app.DefaultPageSize {
		t.Fatalf("page1 items = %d, want %d", len(first.Items), app.DefaultPageSize)
	}
	if first.NextCursor == "" {
		t.Fatal("expected nextCursor on page 1")
	}

	second, err := svc.List(context.Background(), owner, first.NextCursor)
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(second.Items) != 5 {
		t.Fatalf("page2 items = %d, want 5", len(second.Items))
	}
	if second.NextCursor != "" {
		t.Fatalf("page2 nextCursor should be empty: %q", second.NextCursor)
	}
	// Pages must not overlap.
	seen := map[uuid.UUID]struct{}{}
	for _, d := range first.Items {
		seen[d.ID] = struct{}{}
	}
	for _, d := range second.Items {
		if _, dup := seen[d.ID]; dup {
			t.Fatalf("doc %s appeared on both pages", d.ID)
		}
	}
}

func TestList_OwnerIsolation(t *testing.T) {
	svc, repo, store := newSvc(t)
	now := time.Now()
	seedReady(t, repo, store, owner, now, "mine")
	seedReady(t, repo, store, other, now, "yours")

	page, _ := svc.List(context.Background(), owner, "")
	if len(page.Items) != 1 {
		t.Fatalf("expected exactly 1 doc owned by %s, got %d", owner, len(page.Items))
	}
	if page.Items[0].OwnerID != owner {
		t.Fatalf("owner = %q, want %q", page.Items[0].OwnerID, owner)
	}
}

func TestList_BadCursorReturnsErr(t *testing.T) {
	svc, _, _ := newSvc(t)
	_, err := svc.List(context.Background(), owner, "garbage cursor")
	if !errors.Is(err, app.ErrInvalidCursor) {
		t.Fatalf("err = %v, want ErrInvalidCursor", err)
	}
}

func TestGet_HappyPath(t *testing.T) {
	svc, repo, store := newSvc(t)
	doc := seedReady(t, repo, store, owner, time.Now(), "# Body")

	det, err := svc.Get(context.Background(), owner, doc.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if det.Document.ID != doc.ID {
		t.Fatalf("document ID mismatch")
	}
	if len(det.Artifacts) != 1 || det.Artifacts[0].Kind != domain.ArtifactMarkdown {
		t.Fatalf("artifacts = %+v", det.Artifacts)
	}
}

func TestGet_OwnerIsolationReturns404(t *testing.T) {
	svc, repo, store := newSvc(t)
	doc := seedReady(t, repo, store, owner, time.Now(), "x")
	_, err := svc.Get(context.Background(), other, doc.ID)
	if !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("err = %v, want ErrDocumentNotFound", err)
	}
}

func TestMarkdownStream_HappyPath(t *testing.T) {
	svc, repo, store := newSvc(t)
	doc := seedReady(t, repo, store, owner, time.Now(), "# Hello")

	rc, size, err := svc.MarkdownStream(context.Background(), owner, doc.ID)
	if err != nil {
		t.Fatalf("MarkdownStream: %v", err)
	}
	defer rc.Close()
	if size != int64(len("# Hello")) {
		t.Fatalf("size = %d", size)
	}
	body, _ := io.ReadAll(rc)
	if string(body) != "# Hello" {
		t.Fatalf("body = %q", body)
	}
}

func TestMarkdownStream_NotReadyReturnsDomainError(t *testing.T) {
	svc, repo, store := newSvc(t)
	doc := seedReady(t, repo, store, owner, time.Now(), "x")
	doc.Status = domain.StatusQueued
	repo.docs[doc.ID] = doc

	_, _, err := svc.MarkdownStream(context.Background(), owner, doc.ID)
	if !errors.Is(err, app.ErrNotReady) {
		t.Fatalf("err = %v, want ErrNotReady", err)
	}
}

func TestMarkdownStream_OwnerIsolation(t *testing.T) {
	svc, repo, store := newSvc(t)
	doc := seedReady(t, repo, store, owner, time.Now(), "x")
	_, _, err := svc.MarkdownStream(context.Background(), other, doc.ID)
	if !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("err = %v, want ErrDocumentNotFound", err)
	}
}

func TestThumbnailStream_NotFoundWhenNoArtifact(t *testing.T) {
	svc, repo, store := newSvc(t)
	doc := seedReady(t, repo, store, owner, time.Now(), "x")
	_, _, err := svc.ThumbnailStream(context.Background(), owner, doc.ID)
	if !errors.Is(err, app.ErrArtifactNotFound) {
		t.Fatalf("err = %v, want ErrArtifactNotFound", err)
	}
}

func TestRawPresignedURL_HappyPath(t *testing.T) {
	svc, repo, store := newSvc(t)
	doc := seedReady(t, repo, store, owner, time.Now(), "x")
	url, err := svc.RawPresignedURL(context.Background(), owner, doc.ID)
	if err != nil {
		t.Fatalf("RawPresignedURL: %v", err)
	}
	if url.URL == "" {
		t.Fatal("url empty")
	}
	if time.Until(url.ExpiresAt) <= 0 {
		t.Fatalf("expiresAt in the past: %v", url.ExpiresAt)
	}
}

func TestRawPresignedURL_OwnerIsolation(t *testing.T) {
	svc, repo, store := newSvc(t)
	doc := seedReady(t, repo, store, owner, time.Now(), "x")
	_, err := svc.RawPresignedURL(context.Background(), other, doc.ID)
	if !errors.Is(err, domain.ErrDocumentNotFound) {
		t.Fatalf("err = %v, want ErrDocumentNotFound", err)
	}
}
