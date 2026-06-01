package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tomeku/doclens/apps/api/internal/pubsub"
	"github.com/tomeku/doclens/apps/api/internal/transport"
	"github.com/tomeku/doclens/services/shared/auth"
)

func TestStreamDocuments_FlushesEventsAndHeartbeat(t *testing.T) {
	hub := pubsub.NewHub(8)
	srv := New(Deps{Hub: hub})

	rr := newFlushingRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/documents/stream", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = injectIdentity(ctx, auth.Identity{UserID: "u-1", Email: "u@example.com"})
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() {
		srv.StreamDocuments(rr, req)
		close(done)
	}()

	// Give the handler a beat to subscribe and write the connect comment.
	if !waitFor(t, 250*time.Millisecond, func() bool {
		return hub.Subscribers("u-1") == 1
	}) {
		t.Fatalf("subscriber never registered: subs=%d", hub.Subscribers("u-1"))
	}

	hub.Publish(pubsub.Event{
		Op:         "UPDATE",
		OwnerID:    "u-1",
		DocumentID: "00000000-0000-0000-0000-000000000001",
		Status:     "ready",
		UpdatedAt:  time.Date(2026, 6, 1, 10, 30, 0, 0, time.UTC),
	})

	// Wait for the data: line.
	if !waitFor(t, time.Second, func() bool {
		return strings.Contains(rr.body(), "data: ")
	}) {
		t.Fatalf("never saw data: line in body:\n%s", rr.body())
	}

	cancel()
	<-done

	body := rr.body()
	if !strings.HasPrefix(body, ":connected\n\n") {
		t.Fatalf("missing :connected preamble: %q", body[:min(40, len(body))])
	}

	idx := strings.Index(body, "data: ")
	if idx < 0 {
		t.Fatalf("data: line missing")
	}
	rawJSON := body[idx+len("data: "):]
	rawJSON = rawJSON[:strings.Index(rawJSON, "\n\n")]

	var got map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &got); err != nil {
		t.Fatalf("decode JSON %q: %v", rawJSON, err)
	}
	if got["documentId"] != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("documentId = %v", got["documentId"])
	}
	if got["status"] != "ready" {
		t.Fatalf("status = %v", got["status"])
	}
	if got["event"] != "UPDATE" {
		t.Fatalf("event = %v", got["event"])
	}

	// Verify the response headers we set.
	if ct := rr.header.Get("Content-Type"); ct != "text/event-stream; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	if cc := rr.header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("cache-control = %q", cc)
	}
}

func TestStreamDocuments_503WhenHubMissing(t *testing.T) {
	srv := New(Deps{Hub: nil})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/documents/stream", nil).
		WithContext(injectIdentity(context.Background(),
			auth.Identity{UserID: "u-1", Email: "u@example.com"}))

	srv.StreamDocuments(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestStreamDocuments_ScopedByOwner(t *testing.T) {
	hub := pubsub.NewHub(8)
	srv := New(Deps{Hub: hub})

	rr := newFlushingRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/documents/stream", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = injectIdentity(ctx, auth.Identity{UserID: "u-A", Email: "a@example.com"})
	req = req.WithContext(ctx)

	done := make(chan struct{})
	go func() { srv.StreamDocuments(rr, req); close(done) }()

	if !waitFor(t, 250*time.Millisecond, func() bool {
		return hub.Subscribers("u-A") == 1
	}) {
		t.Fatalf("subscriber not registered")
	}

	// Publish for OTHER user.
	hub.Publish(pubsub.Event{
		Op: "UPDATE", OwnerID: "u-B", DocumentID: "d-1", Status: "ready",
		UpdatedAt: time.Now().UTC(),
	})
	// Give the goroutine a moment; should not see anything beyond
	// the :connected preamble.
	time.Sleep(80 * time.Millisecond)

	cancel()
	<-done

	if strings.Contains(rr.body(), "data: ") {
		t.Fatalf("user u-A received an event meant for u-B: %q", rr.body())
	}
}

// --- helpers --------------------------------------------------------------

func injectIdentity(ctx context.Context, id auth.Identity) context.Context {
	// Use the AuthMiddleware path so we don't need to touch unexported
	// keys directly. We set a fake authenticator that returns id, then
	// run a no-op handler under the middleware to get the populated ctx.
	var captured context.Context
	mw := transport.AuthMiddleware(fakeAuth{id: id})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer ignored")
	rr := httptest.NewRecorder()
	mw(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		captured = req.Context()
	})).ServeHTTP(rr, r)
	if captured == nil {
		panic("auth middleware did not invoke next")
	}
	// Splice the auth context into the caller-provided parent so
	// cancellation flows still work.
	return mergedContext{auth: captured, parent: ctx}
}

// mergedContext lets us combine cancellation from a test-controlled
// parent with values from the auth middleware's child context.
type mergedContext struct {
	auth   context.Context
	parent context.Context
}

func (m mergedContext) Deadline() (time.Time, bool) { return m.parent.Deadline() }
func (m mergedContext) Done() <-chan struct{}       { return m.parent.Done() }
func (m mergedContext) Err() error                  { return m.parent.Err() }
func (m mergedContext) Value(key any) any {
	if v := m.auth.Value(key); v != nil {
		return v
	}
	return m.parent.Value(key)
}

type fakeAuth struct{ id auth.Identity }

func (f fakeAuth) Verify(_ context.Context, _ string) (auth.Identity, error) {
	return f.id, nil
}

// flushingRecorder is a chunk-friendly response recorder. The default
// httptest.ResponseRecorder buffers the body but doesn't expose
// Flush(); we need it to satisfy http.Flusher in the handler.
type flushingRecorder struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	header http.Header
	status int
}

func newFlushingRecorder() *flushingRecorder {
	return &flushingRecorder{header: make(http.Header)}
}

func (f *flushingRecorder) Header() http.Header { return f.header }

func (f *flushingRecorder) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(p)
}

func (f *flushingRecorder) WriteHeader(code int) { f.status = code }

func (f *flushingRecorder) Flush() {}

func (f *flushingRecorder) body() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.String()
}

// waitFor polls predicate up to d, returning true on success.
func waitFor(t *testing.T, d time.Duration, pred func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if pred() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return pred()
}

// silence unused imports — io is referenced by other tests in the
// package; keep tooling happy if this file lives alongside them.
var _ = io.EOF
var _ = fmt.Sprintf
