package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/apps/api/internal/handlers"
	"github.com/tomeku/doclens/apps/api/internal/server"
	ingapp "github.com/tomeku/doclens/services/ingestion/app"
	ingdomain "github.com/tomeku/doclens/services/ingestion/domain"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/auth/local"
	extractiondomain "github.com/tomeku/doclens/services/extraction/domain"
	jobsinmem "github.com/tomeku/doclens/services/shared/jobs/inmem"
	"github.com/tomeku/doclens/services/shared/storage"
)

// --- in-memory fakes (mirror of services/ingestion/app/upload_test.go) ----

type uploadRow = ingdomain.Upload
type docRow = libdomain.Document

type uploadRepo struct {
	rows map[uuid.UUID]*uploadRow
}

func newUploadRepo() *uploadRepo { return &uploadRepo{rows: map[uuid.UUID]*uploadRow{}} }
func (r *uploadRepo) Insert(_ context.Context, u *uploadRow) error {
	cp := *u
	r.rows[u.ID] = &cp
	return nil
}
func (r *uploadRepo) FindByID(_ context.Context, ownerID string, id uuid.UUID) (*uploadRow, error) {
	u, ok := r.rows[id]
	if !ok || u.OwnerID != ownerID {
		return nil, ingdomain.ErrUploadNotFound
	}
	cp := *u
	return &cp, nil
}
func (r *uploadRepo) MarkFinalized(_ context.Context, id, doc uuid.UUID, at time.Time) error {
	u, ok := r.rows[id]
	if !ok {
		return ingdomain.ErrUploadNotFound
	}
	u.Status = ingdomain.UploadStatusFinalized
	u.DocumentID = &doc
	u.FinalizedAt = &at
	return nil
}
func (r *uploadRepo) ListExpired(context.Context, time.Time, int) ([]*uploadRow, error) {
	return nil, nil
}
func (r *uploadRepo) MarkExpired(context.Context, []uuid.UUID) error { return nil }

type libRepo struct {
	bySHA     map[string]*docRow
	byID      map[uuid.UUID]*docRow
	artifacts map[string]libdomain.Artifact // key = docID + "/" + kind
}

func newLibRepo() *libRepo {
	return &libRepo{
		bySHA:     map[string]*docRow{},
		byID:      map[uuid.UUID]*docRow{},
		artifacts: map[string]libdomain.Artifact{},
	}
}
func (r *libRepo) FindAliveByOwnerSHA(_ context.Context, ownerID, sha string) (*docRow, error) {
	d, ok := r.bySHA[ownerID+":"+sha]
	if !ok {
		return nil, libdomain.ErrDocumentNotFound
	}
	return d, nil
}
func (r *libRepo) Insert(_ context.Context, d *docRow) error {
	key := d.OwnerID + ":" + d.SHA256
	if _, exists := r.bySHA[key]; exists {
		return libdomain.ErrDuplicateDocument
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	cp := *d
	r.bySHA[key] = &cp
	r.byID[d.ID] = &cp
	return nil
}
func (r *libRepo) FindByID(_ context.Context, ownerID string, id uuid.UUID) (*docRow, error) {
	d, ok := r.byID[id]
	if !ok || d.OwnerID != ownerID {
		return nil, libdomain.ErrDocumentNotFound
	}
	return d, nil
}
func (r *libRepo) FindByIDUnscoped(_ context.Context, id uuid.UUID) (*docRow, error) {
	d, ok := r.byID[id]
	if !ok {
		return nil, libdomain.ErrDocumentNotFound
	}
	return d, nil
}
func (r *libRepo) MarkExtracting(_ context.Context, id uuid.UUID) error {
	d, ok := r.byID[id]
	if !ok {
		return libdomain.ErrDocumentNotFound
	}
	d.Status = libdomain.StatusExtracting
	return nil
}
func (r *libRepo) MarkReady(_ context.Context, id uuid.UUID, m libdomain.ReadyMetrics) error {
	d, ok := r.byID[id]
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
func (r *libRepo) MarkFailed(_ context.Context, id uuid.UUID, reason string) error {
	d, ok := r.byID[id]
	if !ok {
		return libdomain.ErrDocumentNotFound
	}
	d.Status = libdomain.StatusFailed
	rs := reason
	d.LastError = &rs
	return nil
}
func (r *libRepo) MarkRetry(_ context.Context, ownerID string, id uuid.UUID) error {
	d, ok := r.byID[id]
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
func (r *libRepo) UpsertArtifact(_ context.Context, a *libdomain.Artifact) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	r.artifacts[a.DocumentID.String()+"/"+string(a.Kind)] = *a
	return nil
}
func (r *libRepo) SoftDelete(_ context.Context, _ string, _ uuid.UUID) (string, []string, error) {
	return "", nil, errors.New("not used by upload tests")
}
func (r *libRepo) HardDelete(_ context.Context, _ uuid.UUID) error {
	return errors.New("not used by upload tests")
}
func (r *libRepo) ListByOwner(_ context.Context, ownerID string, limit int, cursor *libdomain.Cursor) ([]*docRow, *libdomain.Cursor, error) {
	docs := make([]*docRow, 0)
	for _, d := range r.byID {
		if d.OwnerID == ownerID && d.Status != libdomain.StatusDeleted {
			docs = append(docs, d)
		}
	}
	// Sort by (CreatedAt desc, ID desc).
	sortDocs(docs)
	if cursor != nil {
		filtered := docs[:0]
		for _, d := range docs {
			if d.CreatedAt.Before(cursor.CreatedAt) ||
				(d.CreatedAt.Equal(cursor.CreatedAt) && d.ID.String() < cursor.ID.String()) {
				filtered = append(filtered, d)
			}
		}
		docs = filtered
	}
	var next *libdomain.Cursor
	if len(docs) > limit {
		last := docs[limit-1]
		next = &libdomain.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		docs = docs[:limit]
	}
	return docs, next, nil
}
func (r *libRepo) FindArtifacts(_ context.Context, id uuid.UUID) ([]*libdomain.Artifact, error) {
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

func sortDocs(docs []*docRow) {
	// Insertion sort — lists are small in tests.
	for i := 1; i < len(docs); i++ {
		for j := i; j > 0; j-- {
			a, b := docs[j-1], docs[j]
			if a.CreatedAt.After(b.CreatedAt) {
				break
			}
			if a.CreatedAt.Equal(b.CreatedAt) && a.ID.String() >= b.ID.String() {
				break
			}
			docs[j-1], docs[j] = b, a
		}
	}
}

type fakeStore struct {
	headInfo storage.ObjectInfo
	headErr  error
}

func (f *fakeStore) PresignPut(_ context.Context, bucket, key string, opts storage.PresignPutOptions) (storage.PresignedURL, error) {
	return storage.PresignedURL{
		URL:       "https://signed.example/" + bucket + "/" + key,
		ExpiresAt: time.Now().Add(opts.TTL),
	}, nil
}
func (f *fakeStore) PresignGet(context.Context, string, string, time.Duration) (storage.PresignedURL, error) {
	return storage.PresignedURL{}, errors.New("not implemented")
}
func (f *fakeStore) Head(_ context.Context, _, _ string) (storage.ObjectInfo, error) {
	return f.headInfo, f.headErr
}
func (f *fakeStore) Delete(context.Context, string, string) error { return nil }
func (f *fakeStore) Get(context.Context, string, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented in this test")
}
func (f *fakeStore) Put(context.Context, string, string, io.Reader, storage.PutOptions) error {
	return errors.New("not implemented in this test")
}

// --- fixture --------------------------------------------------------------

const (
	authHeader = "Bearer dev:user_42:alice@example.com"
	otherUser  = "Bearer dev:user_99:eve@example.com"
)

func newUploadServer(t *testing.T, store *fakeStore) (*httptest.Server, *uploadRepo, *libRepo, *jobsinmem.Bus) {
	t.Helper()
	uploads := newUploadRepo()
	library := newLibRepo()
	bus := jobsinmem.NewBus()
	svc := ingapp.NewServiceMust(uploads, library, store, "raw", ingapp.Options{
		PresignTTL:  3 * time.Minute,
		EnabledMime: []string{"application/pdf"},
		Bus:         bus,
	})
	deps := server.Deps{
		Auth:     local.New(),
		Handlers: handlers.Deps{Uploads: svc},
	}
	return httptest.NewServer(server.New(deps)), uploads, library, bus
}

func postJSON(t *testing.T, ts *httptest.Server, path, auth string, body any) *http.Response {
	t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

const validSHA = "0000000000000000000000000000000000000000000000000000000000000001"

// --- tests ----------------------------------------------------------------

func TestCreateUpload_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts, _, _, _ := newUploadServer(t, &fakeStore{})
	defer ts.Close()
	resp := postJSON(t, ts, "/v1/uploads", "", map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf", "size": 10, "sha256": validSHA,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestCreateUpload_Created(t *testing.T) {
	t.Parallel()
	ts, _, _, _ := newUploadServer(t, &fakeStore{})
	defer ts.Close()
	resp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "report.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var body struct {
		DocumentID string  `json:"documentId"`
		PutURL     *string `json:"putUrl"`
		Status     string  `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.PutURL == nil || !strings.Contains(*body.PutURL, "raw/user_42") {
		t.Fatalf("putUrl missing or not owner-scoped: %v", body.PutURL)
	}
	if body.Status != "pending" {
		t.Fatalf("status = %q, want pending", body.Status)
	}
}

func TestCreateUpload_DuplicateReturns200(t *testing.T) {
	t.Parallel()
	ts, _, library, _ := newUploadServer(t, &fakeStore{})
	defer ts.Close()

	existing := &libdomain.Document{
		ID: uuid.New(), OwnerID: "user_42", SHA256: validSHA, Status: libdomain.StatusReady,
	}
	library.bySHA["user_42:"+validSHA] = existing
	library.byID[existing.ID] = existing

	resp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "report.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		DocumentID string  `json:"documentId"`
		PutURL     *string `json:"putUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.DocumentID != existing.ID.String() {
		t.Fatalf("documentId = %s, want %s", body.DocumentID, existing.ID)
	}
	if body.PutURL != nil {
		t.Fatalf("putUrl should be nil on duplicate, got %v", *body.PutURL)
	}
}

func TestCreateUpload_415OnDisallowedMime(t *testing.T) {
	t.Parallel()
	ts, _, _, _ := newUploadServer(t, &fakeStore{})
	defer ts.Close()
	resp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "doc.docx", "mimeType": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"size": 1024, "sha256": validSHA,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/problem+json") {
		t.Fatalf("content-type = %q, want problem+json", got)
	}
}

func TestCreateUpload_413OnOversize(t *testing.T) {
	t.Parallel()
	ts, _, _, _ := newUploadServer(t, &fakeStore{})
	defer ts.Close()
	resp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "big.pdf", "mimeType": "application/pdf",
		"size": ingdomain.MaxUploadBytes + 1, "sha256": validSHA,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413", resp.StatusCode)
	}
}

func TestCreateUpload_400OnBadSHA(t *testing.T) {
	t.Parallel()
	ts, _, _, _ := newUploadServer(t, &fakeStore{})
	defer ts.Close()
	resp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf",
		"size": 1024, "sha256": "not-a-real-hash",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestFinalize_HappyPath(t *testing.T) {
	t.Parallel()
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: 1024}}
	ts, _, _, _ := newUploadServer(t, store)
	defer ts.Close()

	// Step 1: create upload to get an uploadId.
	createResp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	var created struct {
		UploadID   string `json:"uploadId"`
		DocumentID string `json:"documentId"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	createResp.Body.Close()

	// Step 2: finalize.
	finReq, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v1/documents/%s/finalize", ts.URL, created.UploadID),
		nil)
	finReq.Header.Set("Authorization", authHeader)
	finResp, err := http.DefaultClient.Do(finReq)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	defer finResp.Body.Close()
	if finResp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", finResp.StatusCode)
	}
	var doc struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(finResp.Body).Decode(&doc); err != nil {
		t.Fatalf("decode finalize: %v", err)
	}
	if doc.Status != "queued" {
		t.Fatalf("status = %q, want queued", doc.Status)
	}
}

func TestFinalize_OwnerIsolationReturns404(t *testing.T) {
	t.Parallel()
	// Property 2: another user gets 404, not 403, per Req 7.9.
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: 1024}}
	ts, _, _, _ := newUploadServer(t, store)
	defer ts.Close()

	createResp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	var created struct {
		UploadID string `json:"uploadId"`
	}
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v1/documents/%s/finalize", ts.URL, created.UploadID),
		nil)
	req.Header.Set("Authorization", otherUser)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestFinalize_ConflictOnMissingObject(t *testing.T) {
	t.Parallel()
	store := &fakeStore{headErr: storage.ErrNotFound}
	ts, _, _, _ := newUploadServer(t, store)
	defer ts.Close()

	createResp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	var created struct {
		UploadID string `json:"uploadId"`
	}
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/v1/documents/%s/finalize", ts.URL, created.UploadID),
		nil)
	req.Header.Set("Authorization", authHeader)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

func TestUploadServiceUnavailableWhenStorageDown(t *testing.T) {
	t.Parallel()
	// Boot without an upload service to simulate storage offline.
	deps := server.Deps{Auth: local.New()}
	ts := httptest.NewServer(server.New(deps))
	defer ts.Close()

	resp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}


func TestFinalize_EnqueuesExtractionJob(t *testing.T) {
	t.Parallel()
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: 1024}}
	ts, _, _, bus := newUploadServer(t, store)
	defer ts.Close()

	createResp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	var created struct {
		UploadID   string `json:"uploadId"`
		DocumentID string `json:"documentId"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	createResp.Body.Close()

	finReq, _ := http.NewRequest(http.MethodPost,
		ts.URL+"/v1/documents/"+created.UploadID+"/finalize", nil)
	finReq.Header.Set("Authorization", authHeader)
	finResp, err := http.DefaultClient.Do(finReq)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	finResp.Body.Close()
	if finResp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", finResp.StatusCode)
	}

	tasks := bus.Tasks()
	if len(tasks) != 1 {
		t.Fatalf("enqueued %d tasks, want exactly 1", len(tasks))
	}
	if tasks[0].Type != extractiondomain.TaskTypeExtractDocument {
		t.Fatalf("task type = %q, want %q",
			tasks[0].Type, extractiondomain.TaskTypeExtractDocument)
	}
	payload, ok := tasks[0].Payload.(extractiondomain.ExtractDocumentPayload)
	if !ok {
		t.Fatalf("payload type = %T, want ExtractDocumentPayload", tasks[0].Payload)
	}
	if payload.DocumentID != created.DocumentID {
		t.Fatalf("payload.DocumentID = %q, want %q",
			payload.DocumentID, created.DocumentID)
	}
	if payload.OwnerID != "user_42" {
		t.Fatalf("payload.OwnerID = %q, want user_42", payload.OwnerID)
	}
}


// Helper for retry tests: run create + finalize, then mark the doc
// 'failed' inside the in-memory library so the retry endpoint has a
// candidate.
func seedFailedDocument(t *testing.T, ts *httptest.Server, library *libRepo) string {
	t.Helper()
	createResp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	defer createResp.Body.Close()
	var created struct {
		UploadID   string `json:"uploadId"`
		DocumentID string `json:"documentId"`
	}
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	finReq, _ := http.NewRequest(http.MethodPost,
		ts.URL+"/v1/documents/"+created.UploadID+"/finalize", nil)
	finReq.Header.Set("Authorization", authHeader)
	finResp, err := http.DefaultClient.Do(finReq)
	if err != nil {
		t.Fatalf("finalize: %v", err)
	}
	finResp.Body.Close()

	docID := uuid.MustParse(created.DocumentID)
	d := library.byID[docID]
	d.Status = libdomain.StatusFailed
	reason := "test failure"
	d.LastError = &reason
	return created.DocumentID
}

func TestRetry_HappyPath(t *testing.T) {
	t.Parallel()
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: 1024}}
	ts, _, library, bus := newUploadServer(t, store)
	defer ts.Close()

	docID := seedFailedDocument(t, ts, library)

	req, _ := http.NewRequest(http.MethodPost,
		ts.URL+"/v1/documents/"+docID+"/retry", nil)
	req.Header.Set("Authorization", authHeader)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", resp.StatusCode)
	}
	var body struct{ Status string }
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "queued" {
		t.Fatalf("status = %q, want queued", body.Status)
	}
	// Two enqueues total: original finalize + retry. The unique key
	// is owner+docID; the dedupe TTL on the in-mem bus prevents the
	// second one. So we expect exactly 1 task in the bus's record.
	if len(bus.Tasks()) != 1 {
		t.Fatalf("bus tasks = %d, want 1 (deduped retry)", len(bus.Tasks()))
	}
}

func TestRetry_NotFailedReturns409(t *testing.T) {
	t.Parallel()
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: 1024}}
	ts, _, library, _ := newUploadServer(t, store)
	defer ts.Close()

	// Create + finalize, leave status=queued.
	createResp := postJSON(t, ts, "/v1/uploads", authHeader, map[string]any{
		"filename": "x.pdf", "mimeType": "application/pdf", "size": 1024, "sha256": validSHA,
	})
	var created struct {
		UploadID   string `json:"uploadId"`
		DocumentID string `json:"documentId"`
	}
	json.NewDecoder(createResp.Body).Decode(&created)
	createResp.Body.Close()

	finReq, _ := http.NewRequest(http.MethodPost,
		ts.URL+"/v1/documents/"+created.UploadID+"/finalize", nil)
	finReq.Header.Set("Authorization", authHeader)
	finResp, _ := http.DefaultClient.Do(finReq)
	finResp.Body.Close()

	req, _ := http.NewRequest(http.MethodPost,
		ts.URL+"/v1/documents/"+created.DocumentID+"/retry", nil)
	req.Header.Set("Authorization", authHeader)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	_ = library
}

func TestRetry_OwnerIsolationReturns404(t *testing.T) {
	t.Parallel()
	store := &fakeStore{headInfo: storage.ObjectInfo{ByteSize: 1024}}
	ts, _, library, _ := newUploadServer(t, store)
	defer ts.Close()
	docID := seedFailedDocument(t, ts, library)

	req, _ := http.NewRequest(http.MethodPost,
		ts.URL+"/v1/documents/"+docID+"/retry", nil)
	req.Header.Set("Authorization", otherUser)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestRetry_RequiresAuth(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	ts, _, _, _ := newUploadServer(t, store)
	defer ts.Close()
	id := uuid.NewString()
	req, _ := http.NewRequest(http.MethodPost,
		ts.URL+"/v1/documents/"+id+"/retry", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
