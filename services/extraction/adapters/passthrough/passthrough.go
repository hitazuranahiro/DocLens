// Package passthrough provides an Extractor that treats the input
// bytes as already-Markdown.
//
// It exists for two reasons:
//   - Unit/integration tests that exercise the worker plumbing
//     without dragging Python+MarkItDown into the test image.
//   - Local dev where a contributor wants to verify the queue ->
//     handler -> DB path without setting up MarkItDown.
//
// It is never wired into production builds.
//
// Real input bytes (e.g. a PDF) are NOT valid UTF-8. The downstream
// search indexer's body column is a text type that rejects non-UTF8
// bytes, so we sanitize on the way out: invalid runs become U+FFFD
// and embedded NULs are stripped. The result is non-zero text any
// downstream pipeline can handle, even if the meaning is meaningless.
package passthrough

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/tomeku/doclens/services/extraction/domain"
)

// Extractor reads the source as UTF-8 Markdown verbatim.
type Extractor struct{}

// New returns an Extractor.
func New() *Extractor { return &Extractor{} }

// Extract implements domain.Extractor.
func (e *Extractor) Extract(_ context.Context, src io.Reader, hint domain.MimeHint) (*domain.Result, error) {
	bytes, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("passthrough: read: %w", err)
	}
	body := strings.ReplaceAll(strings.ToValidUTF8(string(bytes), "\uFFFD"), "\x00", "")
	// Prepend a synthetic header so the downstream UI shows something
	// recognizable when the dev passthrough adapter is used against
	// a binary input. Production never sees this branch.
	header := "# " + hint.Filename + " (passthrough)\n\n"
	return &domain.Result{
		Markdown:   header + body,
		Metadata:   map[string]any{"engine": "passthrough"},
		Pages:      0,
		Confidence: 100,
	}, nil
}
