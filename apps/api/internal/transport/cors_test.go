package transport_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tomeku/doclens/apps/api/internal/transport"
)

func TestCORS_AllowsConfiguredOrigin(t *testing.T) {
	mw := transport.CORS(transport.CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000"},
	})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/uploads", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("Allow-Origin = %q, want http://localhost:3000", got)
	}
	if got := rr.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Allow-Credentials = %q, want true", got)
	}
	if got := rr.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q, want Origin", got)
	}
}

func TestCORS_RejectsUnknownOrigin(t *testing.T) {
	mw := transport.CORS(transport.CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000"},
	})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/uploads", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Allow-Origin = %q for unknown origin; should be empty", got)
	}
}

func TestCORS_PreflightShortCircuits(t *testing.T) {
	called := false
	mw := transport.CORS(transport.CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000"},
	})
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/v1/uploads", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "authorization, content-type")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rr.Code)
	}
	if called {
		t.Fatalf("preflight should short-circuit before the next handler runs")
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("preflight missing Allow-Methods header")
	}
}

func TestCORS_WildcardAllowsAny(t *testing.T) {
	mw := transport.CORS(transport.CORSConfig{
		AllowedOrigins: []string{"*"},
	})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("Origin", "http://anywhere.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "http://anywhere.example" {
		t.Fatalf("wildcard should mirror any origin; got %q", got)
	}
}

func TestCORS_EmptyAllowListIsPassthrough(t *testing.T) {
	mw := transport.CORS(transport.CORSConfig{})
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("empty allow-list should not emit Allow-Origin; got %q", got)
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}
