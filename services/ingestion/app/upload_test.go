package app_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/services/ingestion/app"
	ingdomain "github.com/tomeku/doclens/services/ingestion/domain"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/storage"
)

// --- fakes ----------------------------------------------------------------

type fakeUploadRepo struct {
	inserts []ingdomain.Upload
	rows    map[uuid.UUID]*ingdomain.Upload
}

func newFakeUploadRepo() *fakeUploadRepo {
	return &fakeUploadRepo{rows: map[uuid.UUID]*ingdomain.Upload{}}
}
func (f *fakeUploadRepo) Insert(_ context.Context, u *ingdomain.Upload) error {
	f.inserts = append(f.inserts, *u)
	cp := *u
	f.rows[u.ID] = &cp
	return nil
}
func (f *fakeUploadRepo) FindByID(_ context.Context, ownerID string, id uuid.UUID) (*ingdomain.Upload, error) {
	u, ok := f.rows[id]
	if !ok || u.OwnerID != ownerID {
		return nil, ingdomain.ErrUploadNotFound
	}
	cp := *u
	return &cp, nil
}
func (f *fakeUploadRepo) MarkFinalized(_ context.Context, id, docID uuid.UUID, at time.Time) error {
	u, ok := f.rows[id]
	if !ok {
		return ingdomain.ErrUploadNotFound
	}
	u.Status = ingdomain.UploadStatusFinalized
	u.DocumentID = &docID
	u.FinalizedAt = &at
	return nil
}
func (f *fakeUploadRepo) ListExpired(context.Context, time.Time, int) ([]*ingdomain.Upload, error) {
	return nil, nil
}
func (f *fakeUploadRepo) MarkExpired(context.Context, []uuid.UUID) error { return nil }

type fakeLibraryRepo struct {
	bySHA map[string]*libdomain.Document
	byID  map[uuid.UUID]*libdomain.Document
}

func newFakeLibraryRepo() *fakeLibraryRepo {
	return &fakeLibraryRepo{
		bySHA: map[string]*libdomain.Document{},
		byID:  map[uuid.UUID]*libdomain.Document{},
	}
}
func (f *fakeLibraryRepo) FindAliveByOwnerSHA(_ context.Context, ownerID, sha string) (*libdomain.Document, error) {
	d, ok := f.bySHA[ownerID+":"+sha]
	if !ok {
		return nil, libdomain.ErrDocumentNotFound
	}
	return d, nil
}
func (f *fakeLibraryRepo) Insert(_ context.Context, d *libdomain.Document) error {
	key := d.OwnerID + ":" + d.SHA256
	if _, exists := f.bySHA[key]; exists {
		return libdomain.ErrDuplicateDocument
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	cp := *d
	f.bySHA[key] = &cp
	f.byID[d.ID] = &cp
	return nil
}
func (f *fakeLibraryRepo) FindByID(_ context.Context, ownerID string, id uuid.UUID) (*libdomain.Document, error) {
	d, ok := f.byID[id]
	if !ok || d.OwnerID != ownerID {
		return nil, libdomain.ErrDocumentNotFound
	}
	return d, nil
}
func (f *fakeLibraryRepo) FindByIDUnscoped(_ context.Context, id uuid.UUID) (*libdomain.Document, error) {
	d, ok := f.byID[id]
	if !ok {
		return nil, libdomain.ErrDocumentNotFound
	}
	return d, nil
}
func (f *fakeLibraryRepo) MarkExtracting(_ context.Context, id uuid.UUID) error {
	d, ok := f.byID[id]
	if !ok {
		return libdomain.ErrDocumentNotFound
	}
	d.Status = libdomain.StatusExtracting
	return nil
}
func (f *fakeLibraryRepo) MarkReady(_ context.Context, id uuid.UUID, m libdomain.ReadyMetrics) error {
	d, ok := f.byID[id]
	if !ok {
		return libdomain.ErrDocumentNotFound
	}
	d.Status = libdomain.StatusReady
	d.PageCount = &m.PageCount
	d.WordCount = &m.WordCount
	d.Confidence = &m.Confidence
	d.LastError = nil
	return nil
}
func (f *fakeLibraryRepo) MarkFailed(_ context.Context, id uuid.UUID, reason string) error {
	d, ok := f.byID[id]
	if !ok {
		return libdomain.ErrDocumentNotFound
	}
	d.Status = libdomain.StatusFailed
	r := reason
	d.LastError = &r
	return nil
}
func (f *fakeLibraryRepo) MarkRetry(_ context.Context, ownerID string, id uuid.UUID) error {
	d, ok := f.byID[id]
	if !ok || d.OwnerID != ownerID {
		return libdomain.ErrDocumentNotFound
	}
	if d.Status != libdomain.StatusFailed {
		return libdomain.ErrInvalidTransition
	}
	d.Status = libdomain.StatusQueued
	d.LastError = nil
	return nil
}
func (f *fakeLibraryRepo) UpsertArtifact(_ context.Context, _ *libdomain.Artifact) error {
	return nil
}

type fakeStore struct {
	headInfo  storage.ObjectInfo
	headErr   error
	puts      []putCall
	deletes   []string
	clockBase time.Time
}

type putCall struct {
	bucket, key string
	opts        storage.PresignPutOptions
}

func (f *fakeStore) PresignPut(_ context.Context, bucket, key string, opts storage.PresignPutOptions) (storage.PresignedURL, error) {
	f.puts = append(f.puts, putCall{bucket, key, opts})
	return storage.PresignedURL{
		URL:       "https://signed.example/" + bucket + "/" + key,
		ExpiresAt: f.clockBase.Add(opts.TTL),
	}, nil
}
func (f *fakeStore) PresignGet(context.Context, string, string, time.Duration) (storage.PresignedURL, error) {
	return storage.PresignedURL{}, errors.New("not implemented")
}
func (f *fakeStore) Head(_ context.Context, _, _ string) (storage.ObjectInfo, error) {
	return f.headInfo, f.headErr
}
func (f *fakeStore) Delete(_ context.Context, bucket, key string) error {
	f.deletes = append(f.deletes, bucket+"/"+key)
	return nil
}
func (f *fakeStore) Get(context.Context, string, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented in this test")
}
func (f *fakeStore) Put(context.Context, string, string, io.Reader, storage.PutOptions) error {
	return errors.New("not implemented in this test")
}

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

// --- helpers --------------------------------------------------------------

const (
	owner   = "user_abc"
	sha     = "0000000000000000000000000000000000000000000000000000000000000001"
	twoSize = int64(1024)
)

func newService(t *testing.T, store storage.ObjectStore) (*app.Service, *fakeUploadRepo, *fakeLibraryRepo) {
	t.Helper()
	uploads := newFakeUploadRepo()
	library := newFakeLibraryRepo()
	clock := fakeClock{now: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)}
	if fs, ok := store.(*fakeStore); ok {
		fs.clockBase = clock.now
	}
	svc, err := app.NewService(uploads, library, store, "doclens-raw", app.Options{
		PresignTTL:  3 * time.Minute,
		EnabledMime: []string{"application/pdf"},
		Clock:       clock,
	})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, uploads, library
}

// --- tests ----------------------------------------------------------------

func TestCreateUpload_FreshUpload_PresignsAndInserts(t *testing.T) {
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: twoSize}}
	svc, uploads, library := newService(t, store)

	res, err := svc.CreateUpload(context.Background(), owner, "doc.pdf", "application/pdf", twoSize, sha, "")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if res.Duplicate {
		t.Fatal("Duplicate should be false on fresh upload")
	}
	if res.PresignedURL == nil {
		t.Fatal("PresignedURL is nil")
	}
	if res.UploadID == nil {
		t.Fatal("UploadID is nil")
	}
	if got, want := len(uploads.inserts), 1; got != want {
		t.Fatalf("uploads inserts = %d, want %d", got, want)
	}
	if !strings.HasPrefix(uploads.inserts[0].ObjectKey, "raw/"+owner+"/") {
		t.Fatalf("object key %q must start with raw/<ownerID>/", uploads.inserts[0].ObjectKey)
	}
	if got, want := len(library.bySHA), 0; got != want {
		t.Fatalf("library should not be touched on createUpload: got %d rows", got)
	}
	if !strings.HasPrefix(store.puts[0].opts.ContentType, "application/pdf") {
		t.Fatalf("presign should bind ContentType, got %q", store.puts[0].opts.ContentType)
	}
}

func TestCreateUpload_Duplicate_ShortCircuits(t *testing.T) {
	store := &fakeStore{}
	svc, uploads, library := newService(t, store)

	existing := &libdomain.Document{
		ID: uuid.New(), OwnerID: owner, SHA256: sha, Status: libdomain.StatusReady,
	}
	library.bySHA[owner+":"+sha] = existing
	library.byID[existing.ID] = existing

	res, err := svc.CreateUpload(context.Background(), owner, "doc.pdf", "application/pdf", twoSize, sha, "")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	if !res.Duplicate {
		t.Fatal("Duplicate should be true")
	}
	if res.PresignedURL != nil {
		t.Fatal("PresignedURL should be nil for duplicates")
	}
	if res.DocumentID != existing.ID {
		t.Fatalf("DocumentID = %v, want %v", res.DocumentID, existing.ID)
	}
	if len(uploads.inserts) != 0 {
		t.Fatal("must not insert an upload row on dedupe")
	}
	if len(store.puts) != 0 {
		t.Fatal("must not call PresignPut on dedupe")
	}
}

func TestCreateUpload_OversizeReturnsDomainError(t *testing.T) {
	svc, _, _ := newService(t, &fakeStore{})
	_, err := svc.CreateUpload(context.Background(), owner, "doc.pdf", "application/pdf",
		ingdomain.MaxUploadBytes+1, sha, "")
	if !errors.Is(err, ingdomain.ErrUploadTooLarge) {
		t.Fatalf("err = %v, want ErrUploadTooLarge", err)
	}
}

func TestCreateUpload_DisallowedMimeReturnsDomainError(t *testing.T) {
	svc, _, _ := newService(t, &fakeStore{})
	_, err := svc.CreateUpload(context.Background(), owner, "doc.docx",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		twoSize, sha, "")
	if !errors.Is(err, ingdomain.ErrUnsupportedMime) {
		t.Fatalf("err = %v, want ErrUnsupportedMime", err)
	}
}

func TestFinalizeUpload_HappyPath(t *testing.T) {
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: twoSize}}
	svc, uploads, library := newService(t, store)

	created, err := svc.CreateUpload(context.Background(), owner, "doc.pdf", "application/pdf", twoSize, sha, "Title")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}

	out, err := svc.FinalizeUpload(context.Background(), owner, *created.UploadID)
	if err != nil {
		t.Fatalf("FinalizeUpload: %v", err)
	}
	if out.Document.Status != libdomain.StatusQueued {
		t.Fatalf("status = %s, want queued", out.Document.Status)
	}
	if got, want := uploads.rows[*created.UploadID].Status, ingdomain.UploadStatusFinalized; got != want {
		t.Fatalf("upload status = %s, want %s", got, want)
	}
	if _, ok := library.byID[out.Document.ID]; !ok {
		t.Fatal("library row was not created")
	}
}

func TestFinalizeUpload_MissingObjectReturnsDomainError(t *testing.T) {
	store := &fakeStore{headErr: storage.ErrNotFound}
	svc, _, _ := newService(t, store)

	created, err := svc.CreateUpload(context.Background(), owner, "doc.pdf", "application/pdf", twoSize, sha, "")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	_, err = svc.FinalizeUpload(context.Background(), owner, *created.UploadID)
	if !errors.Is(err, ingdomain.ErrObjectMissing) {
		t.Fatalf("err = %v, want ErrObjectMissing", err)
	}
}

func TestFinalizeUpload_OwnerIsolation(t *testing.T) {
	// Property 2: another owner can never finalize someone else's upload.
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: twoSize}}
	svc, _, _ := newService(t, store)

	created, err := svc.CreateUpload(context.Background(), owner, "doc.pdf", "application/pdf", twoSize, sha, "")
	if err != nil {
		t.Fatalf("CreateUpload: %v", err)
	}
	_, err = svc.FinalizeUpload(context.Background(), "user_evil", *created.UploadID)
	if !errors.Is(err, ingdomain.ErrUploadNotFound) {
		t.Fatalf("err = %v, want ErrUploadNotFound", err)
	}
}

func TestFinalizeUpload_Idempotent(t *testing.T) {
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: twoSize}}
	svc, _, _ := newService(t, store)

	created, _ := svc.CreateUpload(context.Background(), owner, "doc.pdf", "application/pdf", twoSize, sha, "")
	out1, err := svc.FinalizeUpload(context.Background(), owner, *created.UploadID)
	if err != nil {
		t.Fatalf("first finalize: %v", err)
	}
	out2, err := svc.FinalizeUpload(context.Background(), owner, *created.UploadID)
	if err != nil {
		t.Fatalf("second finalize: %v", err)
	}
	if out1.Document.ID != out2.Document.ID {
		t.Fatalf("idempotent finalize returned different documents: %v vs %v",
			out1.Document.ID, out2.Document.ID)
	}
}
