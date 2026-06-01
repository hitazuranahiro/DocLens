// Package domain defines the Extractor port used by the worker.
//
// The port is engine-agnostic. v0.1 has one adapter (MarkItDown CLI),
// but the interface stays narrow so we can layer a Poppler fallback
// for high-fidelity research-paper mode in v0.2 without changing the
// worker (per ADR 0012).
package domain

import (
	"context"
	"errors"
	"io"
)

// MimeHint tells the extractor what kind of document to expect. The
// MIME type is authoritative; Filename is for log lines and any
// adapter that uses extension as a fallback.
type MimeHint struct {
	MimeType string
	Filename string
}

// Result is what every adapter returns.
//
// Markdown is the canonical extraction output. Metadata holds
// adapter-specific structured fields (PDF info dict, EPUB OPF,
// EXIF, etc.). Pages and Confidence are best-effort; an adapter
// that cannot compute them sets Pages=0 and Confidence=0.
type Result struct {
	Markdown   string
	Metadata   map[string]any
	Pages      int
	Confidence float32
	Warnings   []string
}

// Extractor is the port. Implementations MUST honor ctx cancellation.
//
// Adapters MUST NOT mutate the source reader past EOF; callers may
// reuse it (we don't, today, but the contract is cheap).
type Extractor interface {
	Extract(ctx context.Context, src io.Reader, hint MimeHint) (*Result, error)
}

// Domain errors.
var (
	// ErrUnsupportedMime says the adapter can't handle this format.
	ErrUnsupportedMime = errors.New("extraction: mime not supported by adapter")

	// ErrTimeout fires when the per-attempt deadline elapsed before
	// the adapter produced a result.
	ErrTimeout = errors.New("extraction: timed out")

	// ErrEngineCrashed is for non-recoverable adapter failures
	// (subprocess died, library panicked, etc.). The worker should
	// mark the document failed without retrying.
	ErrEngineCrashed = errors.New("extraction: engine crashed")
)


// TaskTypeExtractDocument is the canonical asynq task name dispatched
// by the API after a document is finalized. Defined in domain so
// callers (the API enqueuer) and consumers (the worker) share one
// constant without depending on each other.
const TaskTypeExtractDocument = "extract.document"

// ExtractDocumentPayload is the JSON shape sent on TaskTypeExtractDocument.
type ExtractDocumentPayload struct {
	DocumentID string `json:"documentId"`
	OwnerID    string `json:"ownerId"`
}
