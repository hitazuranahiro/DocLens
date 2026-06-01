package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	bySHA map[string]*docRow
	byID  map[uuid.UUID]*docRow
}

func newLibRepo() *libRepo {
	return &libRepo{bySHA: map[string]*docRow{}, byID: map[uuid.UUID]*docRow{}}
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

// --- fixture --------------------------------------------------------------

const (
	authHeader = "Bearer dev:user_42:alice@example.com"
	otherUser  = "Bearer dev:user_99:eve@example.com"
)

func newUploadServer(t *testing.T, store *fakeStore) (*httptest.Server, *uploadRepo, *libRepo) {
	t.Helper()
	uploads := newUploadRepo()
	library := newLibRepo()
	svc := ingapp.NewServiceMust(uploads, library, store, "raw", ingapp.Options{
		PresignTTL:  3 * time.Minute,
		EnabledMime: []string{"application/pdf"},
	})
	deps := server.Deps{
		Auth:     local.New(),
		Handlers: handlers.Deps{Uploads: svc},
	}
	return httptest.NewServer(server.New(deps)), uploads, library
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
	ts, _, _ := newUploadServer(t, &fakeStore{})
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
	ts, _, _ := newUploadServer(t, &fakeStore{})
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
	ts, _, library := newUploadServer(t, &fakeStore{})
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
	ts, _, _ := newUploadServer(t, &fakeStore{})
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
	ts, _, _ := newUploadServer(t, &fakeStore{})
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
	ts, _, _ := newUploadServer(t, &fakeStore{})
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
	ts, _, _ := newUploadServer(t, store)
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
	ts, _, _ := newUploadServer(t, store)
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
	ts, _, _ := newUploadServer(t, store)
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
