package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tomeku/doclens/apps/api/internal/pubsub"
	"github.com/tomeku/doclens/apps/api/internal/transport"
)

// StreamDocuments implements GET /v1/documents/stream as a Server-Sent
// Events feed of live document_status updates for the authenticated
// user.
//
// Wire format:
//
//   data: {"event":"UPDATE","documentId":"…","status":"ready", …}\n\n
//   :keepalive\n\n        ← every 25s, keeps proxies from closing
//
// We never send a typed `event:` line in v0.1; the JSON body carries
// the kind. The browser-side EventSource parses this as a default
// `message` event, which keeps the React hook trivial.
func (s *Server) StreamDocuments(w http.ResponseWriter, r *http.Request) {
	if s.hub == nil {
		// Hub wasn't wired (Postgres unavailable at boot). Return 503
		// so the browser falls back to its own polling cadence.
		transport.WriteProblem(w, http.StatusServiceUnavailable,
			"Live updates unavailable",
			"the API is running without a connected pubsub listener")
		return
	}
	id, ok := transport.IdentityFrom(r.Context())
	if !ok {
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		// Should not happen with chi + net/http stdlib, but the
		// cast is the contract; bail with 500 if the response
		// writer can't flush.
		transport.WriteProblem(w, http.StatusInternalServerError,
			"Streaming unsupported", "")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	// Disable buffering on nginx-style proxies. Harmless elsewhere.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	sub := s.hub.Subscribe(id.UserID)
	defer sub.Close()

	// Initial comment kicks the connection alive immediately so
	// EventSource resolves `onopen` without waiting for the first
	// real event.
	if _, err := fmt.Fprint(w, ":connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ":keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, ok := <-sub.C:
			if !ok {
				return
			}
			if err := writeEvent(w, ev); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// writeEvent serializes one Event as a single SSE message. We use
// camelCase JSON to match the rest of the API; the listener decoded
// the snake_case payload from Postgres into a typed Event already.
func writeEvent(w http.ResponseWriter, ev pubsub.Event) error {
	wire := struct {
		Event      string    `json:"event,omitempty"`
		DocumentID string    `json:"documentId"`
		Status     string    `json:"status"`
		PageCount  *int      `json:"pageCount,omitempty"`
		WordCount  *int      `json:"wordCount,omitempty"`
		Confidence *int      `json:"confidence,omitempty"`
		LastError  *string   `json:"lastError,omitempty"`
		UpdatedAt  time.Time `json:"updatedAt"`
	}{
		Event:      ev.Op,
		DocumentID: ev.DocumentID,
		Status:     ev.Status,
		PageCount:  ev.PageCount,
		WordCount:  ev.WordCount,
		Confidence: ev.Confidence,
		LastError:  ev.LastError,
		UpdatedAt:  ev.UpdatedAt,
	}
	body, err := json.Marshal(wire)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", body)
	return err
}
