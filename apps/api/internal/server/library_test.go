package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tomeku/doclens/apps/api/internal/handlers"
	"github.com/tomeku/doclens/apps/api/internal/server"
	libapp "github.com/tomeku/doclens/services/library/app"
	libdomain "github.com/tomeku/doclens/services/library/domain"
	"github.com/tomeku/doclens/services/shared/auth/local"
	"github.com/tomeku/doclens/services/shared/storage"
)

// libStore is a tiny in-memory ObjectStore stand-in used by the
// library handler tests. It supports Get / PresignGet so the four
// new endpoints can be exercised end-to-end.
type libStore struct {
	objects map[string][]byte
}

func newLibStore() *libStore { return &libStore{objects: map[string][]byte{}} }
func (f *libStore) put(bucket, key string, body []byte) {
	f.objects[bucket+"/"+key] = body
}
func (f *libStore) PresignPut(context.Context, string, string, storage.PresignPutOptions) (storage.PresignedURL, error) {
	return storage.PresignedURL{}, errors.New("not used")
}
func (f *libStore) PresignGet(_ context.Context, bucket, key string, ttl time.Duration) (storage.PresignedURL, error) {
	return storage.PresignedURL{
		URL:       "https://signed.example/" + bucket + "/" + key,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}
func (f *libStore) Head(context.Context, string, string) (storage.ObjectInfo, error) {
	return storage.ObjectInfo{}, errors.New("not used")
}
func (f *libStore) Get(_ context.Context, bucket, key string) (io.ReadCloser, error) {
	b, ok := f.objects[bucket+"/"+key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(byteReader(b)), nil
}
func (f *libStore) Put(context.Context, string, string, io.Reader, storage.PutOptions) error {
	return errors.New("not used")
}
func (f *libStore) Delete(context.Context, string, string) error { return nil }

type byteReader []byte

func (b byteReader) Read(p []byte) (int, error) {
	if len(b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, b)
	return n, nil
}

// newLibraryServer builds a server backed by the same in-memory libRepo
// used by the upload tests. We pre-seed N ready documents owned by
// `user_42` plus an extra one owned by `user_99` for owner-isolation
// checks.
func newLibraryServer(t *testing.T, ownerDocs int) (*httptest.Server, *libRepo, *libStore, []*docRow) {
	t.Helper()
	repo := newLibRepo()
	store := newLibStore()

	docs := make([]*docRow, 0, ownerDocs)
	base := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < ownerDocs; i++ {
		id := uuid.New()
		md := []byte("# Doc " + id.String()[:6])
		doc := &libdomain.Document{
			ID:             id,
			OwnerID:        "user_42",
			Title:          "Title",
			SourceFilename: "doc.pdf",
			SHA256:         "0000000000000000000000000000000000000000000000000000000000000001",
			ByteSize:       int64(len(md)),
			MimeType:       "application/pdf",
			Status:         libdomain.StatusReady,
			RawObjectKey:   "raw/user_42/" + id.String() + "/doc.pdf",
			CreatedAt:      base.Add(time.Duration(i) * time.Minute),
			UpdatedAt:      base.Add(time.Duration(i) * time.Minute),
		}
		repo.byID[id] = doc
		repo.bySHA["user_42:"+doc.SHA256+":"+id.String()] = doc

		mdKey := "artifacts/" + id.String() + "/extracted.md"
		repo.artifacts[id.String()+"/"+string(libdomain.ArtifactMarkdown)] = libdomain.Artifact{
			ID:         uuid.New(),
			DocumentID: id,
			Kind:       libdomain.ArtifactMarkdown,
			ObjectKey:  mdKey,
			ByteSize:   int64(len(md)),
		}
		store.put("doclens-artifacts", mdKey, md)
		docs = append(docs, doc)
	}

	// Seed one doc owned by user_99 to verify owner isolation.
	otherID := uuid.New()
	repo.byID[otherID] = &libdomain.Document{
		ID:           otherID,
		OwnerID:      "user_99",
		Title:        "Their doc",
		Status:       libdomain.StatusReady,
		MimeType:     "application/pdf",
		RawObjectKey: "raw/user_99/" + otherID.String() + "/x.pdf",
		CreatedAt:    base,
	}

	libSvc, err := libapp.NewService(repo, store, "doclens-raw", "doclens-artifacts")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	deps := server.Deps{
		Auth: local.New(),
		Handlers: handlers.Deps{
			Library: libSvc,
		},
	}
	return httptest.NewServer(server.New(deps)), repo, store, docs
}

// libRepo's existing helper methods need a small extension. The
// owner_test fakes already implement most of the Repository surface;
// we need ListByOwner and FindArtifacts to return real data here.
//
// We re-shadow ListByOwner and FindArtifacts via a little helper
// repo that wraps the existing libRepo. Done as a separate type so
// the upload tests aren't affected.
//
// (Defined inline above so only this test file uses it.)

func get(t *testing.T, ts *httptest.Server, path, auth string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func TestList_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts, _, _, _ := newLibraryServer(t, 1)
	defer ts.Close()
	resp := get(t, ts, "/v1/documents", "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestList_OwnerScoped(t *testing.T) {
	t.Parallel()
	ts, _, _, docs := newLibraryServer(t, 3)
	defer ts.Close()
	resp := get(t, ts, "/v1/documents", authHeader)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Items []struct {
			ID      string `json:"id"`
			OwnerID string `json:"ownerId"`
		} `json:"items"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(body.Items))
	}
	for _, it := range body.Items {
		if it.OwnerID != "user_42" {
			t.Fatalf("item owner = %q, want user_42", it.OwnerID)
		}
	}
	if body.NextCursor != nil {
		t.Fatalf("nextCursor should be null for a single small page: %v", *body.NextCursor)
	}
	_ = docs
}

func TestList_BadCursor(t *testing.T) {
	t.Parallel()
	ts, _, _, _ := newLibraryServer(t, 1)
	defer ts.Close()
	resp := get(t, ts, "/v1/documents?cursor=$$$invalid$$$", authHeader)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestGet_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _, docs := newLibraryServer(t, 1)
	defer ts.Close()
	resp := get(t, ts, "/v1/documents/"+docs[0].ID.String(), authHeader)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Document  struct{ ID string }
		Artifacts []struct {
			Kind string
		}
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Document.ID != docs[0].ID.String() {
		t.Fatalf("document.id = %q, want %q", body.Document.ID, docs[0].ID)
	}
	if len(body.Artifacts) != 1 || body.Artifacts[0].Kind != "markdown" {
		t.Fatalf("artifacts = %+v", body.Artifacts)
	}
}

func TestGet_OwnerIsolationReturns404(t *testing.T) {
	t.Parallel()
	ts, repo, _, _ := newLibraryServer(t, 0)
	defer ts.Close()
	// user_99 doc seeded in newLibraryServer
	var otherID uuid.UUID
	for id, d := range repo.byID {
		if d.OwnerID == "user_99" {
			otherID = id
			break
		}
	}
	resp := get(t, ts, "/v1/documents/"+otherID.String(), authHeader)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGetMarkdown_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _, docs := newLibraryServer(t, 1)
	defer ts.Close()
	resp := get(t, ts, "/v1/documents/"+docs[0].ID.String()+"/markdown", authHeader)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "text/markdown; charset=utf-8" {
		t.Fatalf("content-type = %q", got)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body)[:5] != "# Doc" {
		t.Fatalf("body = %q...", string(body)[:min(20, len(body))])
	}
}

func TestGetMarkdown_NotReadyReturns409(t *testing.T) {
	t.Parallel()
	ts, repo, _, docs := newLibraryServer(t, 1)
	defer ts.Close()
	// Force the doc into 'queued' so MarkdownStream returns ErrNotReady.
	d := repo.byID[docs[0].ID]
	d.Status = libdomain.StatusQueued

	resp := get(t, ts, "/v1/documents/"+docs[0].ID.String()+"/markdown", authHeader)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

func TestGetThumbnail_NotFoundWhenNoArtifact(t *testing.T) {
	t.Parallel()
	ts, _, _, docs := newLibraryServer(t, 1)
	defer ts.Close()
	resp := get(t, ts, "/v1/documents/"+docs[0].ID.String()+"/thumbnail", authHeader)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGetRawURL_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _, docs := newLibraryServer(t, 1)
	defer ts.Close()
	resp := get(t, ts, "/v1/documents/"+docs[0].ID.String()+"/raw", authHeader)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		URL       string `json:"url"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.URL == "" {
		t.Fatal("url empty")
	}
	exp, err := time.Parse(time.RFC3339Nano, body.ExpiresAt)
	if err != nil {
		t.Fatalf("expiresAt: %v", err)
	}
	if time.Until(exp) <= 0 {
		t.Fatalf("expiresAt in the past")
	}
	// Property 7: TTL ≤ 5 minutes.
	if time.Until(exp) > 5*time.Minute+5*time.Second {
		t.Fatalf("expiresAt too far in the future: %v", exp)
	}
}
