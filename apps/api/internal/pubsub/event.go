// Package pubsub implements the in-process fanout for live document
// status updates (M6).
//
// The flow:
//
//   Postgres NOTIFY  →  Listener (one dedicated pgx.Conn)
//                    →  Hub.Publish (typed Event)
//                    →  every Subscriber whose ownerID matches
//                    →  HTTP SSE handler  →  EventSource on the web
//
// Property: every event lands on every active subscriber for the
// matching owner, or is dropped (with a metric) when the subscriber's
// channel is full. We never block the listener on a slow client.
package pubsub

import "time"

// Event is the typed shape of one document_status notification.
//
// Field names match the JSON the trigger writes (snake_case) so the
// listener can json.Unmarshal directly into this struct. The HTTP
// handler re-serializes to camelCase before sending to the browser
// so the wire format is consistent with the rest of the API.
type Event struct {
	// Op is the trigger operation: "INSERT" | "UPDATE".
	Op string `json:"event"`
	// OwnerID is the Clerk userId from library.documents.owner_id.
	// The hub uses this to scope fanout (Property 2 — owner isolation).
	OwnerID string `json:"owner_id"`
	// DocumentID is the document UUID, in canonical hex form.
	DocumentID string `json:"document_id"`
	// Status mirrors library.document_status.
	Status string `json:"status"`
	// PageCount/WordCount/Confidence/LastError are populated when the
	// extraction worker writes them; nil otherwise.
	PageCount  *int    `json:"page_count,omitempty"`
	WordCount  *int    `json:"word_count,omitempty"`
	Confidence *int    `json:"confidence,omitempty"`
	LastError  *string `json:"last_error,omitempty"`
	// UpdatedAt is the row's timezone-aware updated_at.
	UpdatedAt time.Time `json:"updated_at"`
}
