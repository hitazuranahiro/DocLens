package app_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/extraction/adapters/passthrough"
	"github.com/tomeku/doclens/services/extraction/app"
	"github.com/tomeku/doclens/services/extraction/domain"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/storage"
)

// --- fakes ----------------------------------------------------------------

type libRepo struct {
	mu        sync.Mutex
	docs      map[uuid.UUID]*libdomain.Document
	artifacts map[string]libdomain.Artifact // key = docID + "/" + kind
}

func newLibRepo() *libRepo {
	return &libRepo{
		docs:      map[uuid.UUID]*libdomain.Document{},
		artifacts: map[string]libdomain.Artifact{},
	}
}
func (r *libRepo) FindAliveByOwnerSHA(_ context.Context, _, _ string) (*libdomain.Document, error) {
	return nil, libdomain.ErrDocumentNotFound
}
func (r *libRepo) Insert(_ context.Context, d *libdomain.Document) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	cp := *d
	r.docs[d.ID] = &cp
	return nil
}
func (r *libRepo) FindByID(_ context.Context, ownerID string, id uuid.UUID) (*libdomain.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.docs[id]
	if !ok || d.OwnerID != ownerID {
		return nil, libdomain.ErrDocumentNotFound
	}
	cp := *d
	return &cp, nil
}
func (r *libRepo) FindByIDUnscoped(_ context.Context, id uuid.UUID) (*libdomain.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.docs[id]
	if !ok {
		return nil, libdomain.ErrDocumentNotFound
	}
	cp := *d
	return &cp, nil
}
func (r *libRepo) MarkExtracting(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.docs[id]
	if !ok {
		return libdomain.ErrDocumentNotFound
	}
	if d.Status == libdomain.StatusDeleted ||
		d.Status == libdomain.StatusReady ||
		d.Status == libdomain.StatusFailed {
		return libdomain.ErrInvalidTransition
	}
	d.Status = libdomain.StatusExtracting
	return nil
}
func (r *libRepo) MarkReady(_ context.Context, id uuid.UUID, m libdomain.ReadyMetrics) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.docs[id]
	if !ok {
		return libdomain.ErrDocumentNotFound
	}
	if d.Status == libdomain.StatusDeleted {
		return libdomain.ErrInvalidTransition
	}
	d.Status = libdomain.StatusReady
	pc, wc, cf := m.PageCount, m.WordCount, m.Confidence
	d.PageCount = &pc
	d.WordCount = &wc
	d.Confidence = &cf
	d.LastError = nil
	return nil
}
func (r *libRepo) MarkFailed(_ context.Context, id uuid.UUID, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.docs[id]
	if !ok {
		return libdomain.ErrDocumentNotFound
	}
	if d.Status == libdomain.StatusDeleted {
		return libdomain.ErrInvalidTransition
	}
	d.Status = libdomain.StatusFailed
	rs := reason
	d.LastError = &rs
	return nil
}
func (r *libRepo) MarkRetry(_ context.Context, _ string, _ uuid.UUID) error {
	return errors.New("not used by extraction")
}
func (r *libRepo) UpsertArtifact(_ context.Context, a *libdomain.Artifact) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	r.artifacts[a.DocumentID.String()+"/"+string(a.Kind)] = *a
	return nil
}
func (r *libRepo) ListByOwner(context.Context, string, int, *libdomain.Cursor) ([]*libdomain.Document, *libdomain.Cursor, error) {
	return nil, nil, nil
}
func (r *libRepo) FindArtifacts(_ context.Context, id uuid.UUID) ([]*libdomain.Artifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*libdomain.Artifact, 0)
	prefix := id.String() + "/"
	for k, a := range r.artifacts {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			cp := a
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *libRepo) artifactCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.artifacts)
}

type fakeStore struct {
	mu      sync.Mutex
	objects map[string][]byte // key = bucket + "/" + key
	getErr  error
	putErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}}
}
func (f *fakeStore) put(bucket, key string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[bucket+"/"+key] = body
}
func (f *fakeStore) PresignPut(context.Context, string, string, storage.PresignPutOptions) (storage.PresignedURL, error) {
	return storage.PresignedURL{}, errors.New("not used")
}
func (f *fakeStore) PresignGet(context.Context, string, string, time.Duration) (storage.PresignedURL, error) {
	return storage.PresignedURL{}, errors.New("not used")
}
func (f *fakeStore) Head(context.Context, string, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, errors.New("not used")
}
func (f *fakeStore) Get(_ context.Context, bucket, key string) (io.ReadCloser, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[bucket+"/"+key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (f *fakeStore) Put(_ context.Context, bucket, key string, body io.Reader, _ storage.PutOptions) error {
	if f.putErr != nil {
		return f.putErr
	}
	all, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[bucket+"/"+key] = all
	return nil
}
func (f *fakeStore) Delete(context.Context, string, string) error { return nil }

type fakeExtractor struct {
	result *domain.Result
	err    error
	calls  int
	mu     sync.Mutex
}

func (e *fakeExtractor) Extract(_ context.Context, _ io.Reader, _ domain.MimeHint) (*domain.Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls++
	if e.err != nil {
		return nil, e.err
	}
	return e.result, nil
}

// --- fixtures -------------------------------------------------------------

const (
	rawBucket  = "doclens-raw"
	artBucket  = "doclens-artifacts"
	ownerID    = "user_42"
	rawKey     = "raw/user_42/doc-1/file.pdf"
	rawContent = "%PDF-1.7 fake bytes"
)

func newSeededDoc(t *testing.T, repo *libRepo) *libdomain.Document {
	t.Helper()
	doc := &libdomain.Document{
		ID:             uuid.New(),
		OwnerID:        ownerID,
		Title:          "Report",
		SourceFilename: "file.pdf",
		SHA256:         "0000000000000000000000000000000000000000000000000000000000000001",
		ByteSize:       int64(len(rawContent)),
		MimeType:       "application/pdf",
		Status:         libdomain.StatusQueued,
		RawObjectKey:   rawKey,
	}
	if err := repo.Insert(context.Background(), doc); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	return doc
}

func newService(t *testing.T) (*app.Service, *libRepo, *fakeStore, *fakeExtractor) {
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
	svc, err := app.NewService(repo, store, ext, rawBucket, artBucket, app.Options{
		EnabledMimes: []string{"application/pdf"},
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, repo, store, ext
}

func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

// --- tests ----------------------------------------------------------------

func TestExtract_HappyPath(t *testing.T) {
	svc, repo, store, ext := newService(t)
	doc := newSeededDoc(t, repo)

	if err := svc.Extract(context.Background(), doc.ID); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if ext.calls != 1 {
		t.Fatalf("extractor calls = %d, want 1", ext.calls)
	}

	got, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)
	if got.Status != libdomain.StatusReady {
		t.Fatalf("status = %s, want ready", got.Status)
	}
	if got.PageCount == nil || *got.PageCount != 1 {
		t.Fatalf("PageCount = %v, want 1", got.PageCount)
	}
	if got.Confidence == nil || *got.Confidence < 90 {
		t.Fatalf("Confidence = %v, want >= 90", got.Confidence)
	}
	if got.WordCount == nil || *got.WordCount < 100 {
		t.Fatalf("WordCount = %v, want >= 100", got.WordCount)
	}

	// Markdown artifact persisted.
	mdKey := "artifacts/" + doc.ID.String() + "/extracted.md"
	store.mu.Lock()
	defer store.mu.Unlock()
	body, ok := store.objects[artBucket+"/"+mdKey]
	if !ok {
		t.Fatalf("markdown artifact not stored at %s", mdKey)
	}
	if !bytes.HasPrefix(body, []byte("# Hello")) {
		t.Fatalf("markdown body unexpected: %q", body[:min(40, len(body))])
	}
	if repo.artifactCount() != 1 {
		t.Fatalf("artifact rows = %d, want 1 (no thumbnailer configured)", repo.artifactCount())
	}
}

// Property 3: idempotent extraction.
func TestExtract_IdempotentReRunProducesSameEndState(t *testing.T) {
	svc, repo, store, ext := newService(t)
	doc := newSeededDoc(t, repo)

	if err := svc.Extract(context.Background(), doc.ID); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)
	firstWords := *first.WordCount
	firstConf := *first.Confidence

	// Simulate the worker picking up the same task again. MarkExtracting
	// refuses ready -> extracting (the row is now in 'ready'), so the
	// worker's idempotent path acks early. End-state must be unchanged.
	if err := svc.Extract(context.Background(), doc.ID); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)

	if second.Status != libdomain.StatusReady {
		t.Fatalf("status = %s, want ready", second.Status)
	}
	if *second.WordCount != firstWords {
		t.Fatalf("word count drifted: %d -> %d", firstWords, *second.WordCount)
	}
	if *second.Confidence != firstConf {
		t.Fatalf("confidence drifted: %d -> %d", firstConf, *second.Confidence)
	}
	// Artifact still uniquely keyed by (docID, kind).
	if repo.artifactCount() != 1 {
		t.Fatalf("artifact rows = %d, want 1", repo.artifactCount())
	}
	// Extractor was called only once because the second run no-op'd.
	if ext.calls != 1 {
		t.Fatalf("extractor calls = %d, want 1 (second run should be early-ack)", ext.calls)
	}
	_ = store
}

func TestExtract_DocumentDeletedIsNoop(t *testing.T) {
	svc, repo, _, ext := newService(t)
	id := uuid.New()
	repo.docs[id] = &libdomain.Document{ID: id, Status: libdomain.StatusDeleted}

	if err := svc.Extract(context.Background(), id); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if ext.calls != 0 {
		t.Fatal("extractor should not run on deleted document")
	}
}

func TestExtract_RawObjectMissingMarksFailed(t *testing.T) {
	svc, repo, store, _ := newService(t)
	doc := newSeededDoc(t, repo)
	// Wipe the object before the worker runs.
	store.mu.Lock()
	delete(store.objects, rawBucket+"/"+rawKey)
	store.mu.Unlock()

	if err := svc.Extract(context.Background(), doc.ID); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)
	if got.Status != libdomain.StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if got.LastError == nil || *got.LastError != "raw object missing" {
		t.Fatalf("LastError = %v", got.LastError)
	}
}

func TestExtract_TimeoutMarksFailed(t *testing.T) {
	repo := newLibRepo()
	store := newFakeStore()
	store.put(rawBucket, rawKey, []byte(rawContent))
	ext := &fakeExtractor{err: domain.ErrTimeout}
	svc, _ := app.NewService(repo, store, ext, rawBucket, artBucket, app.Options{
		EnabledMimes: []string{"application/pdf"},
	})

	doc := newSeededDoc(t, repo)
	if err := svc.Extract(context.Background(), doc.ID); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)
	if got.Status != libdomain.StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if got.LastError == nil || *got.LastError != "extraction timed out" {
		t.Fatalf("LastError = %v", got.LastError)
	}
}

func TestExtract_DisallowedMimeMarksFailed(t *testing.T) {
	repo := newLibRepo()
	store := newFakeStore()
	store.put(rawBucket, rawKey, []byte(rawContent))
	ext := &fakeExtractor{result: &domain.Result{Markdown: "x"}}
	svc, _ := app.NewService(repo, store, ext, rawBucket, artBucket, app.Options{
		EnabledMimes: []string{"application/pdf"},
	})

	doc := newSeededDoc(t, repo)
	doc.MimeType = "application/zip"
	repo.docs[doc.ID] = doc

	if err := svc.Extract(context.Background(), doc.ID); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)
	if got.Status != libdomain.StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if ext.calls != 0 {
		t.Fatal("extractor should not run for disallowed mime")
	}
}

func TestExtract_DocumentGoneIsNoop(t *testing.T) {
	svc, _, _, _ := newService(t)
	if err := svc.Extract(context.Background(), uuid.New()); err != nil {
		t.Fatalf("Extract: %v", err)
	}
}

func TestExtract_PassthroughHappyPath(t *testing.T) {
	repo := newLibRepo()
	store := newFakeStore()
	body := "# Already markdown\n\n" + repeat("body ", 150)
	store.put(rawBucket, rawKey, []byte(body))
	svc, _ := app.NewService(repo, store, passthrough.New(), rawBucket, artBucket, app.Options{
		EnabledMimes: []string{"application/pdf"},
	})

	doc := newSeededDoc(t, repo)
	if err := svc.Extract(context.Background(), doc.ID); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got, _ := repo.FindByIDUnscoped(context.Background(), doc.ID)
	if got.Status != libdomain.StatusReady {
		t.Fatalf("status = %s, want ready", got.Status)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
