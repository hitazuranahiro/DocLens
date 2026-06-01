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
package passthrough

import (
	"context"
	"fmt"
	"io"

	"github.com/tomeku/doclens/services/extraction/domain"
)

// Extractor reads the source as UTF-8 Markdown verbatim.
type Extractor struct{}

// New returns an Extractor.
func New() *Extractor { return &Extractor{} }

// Extract implements domain.Extractor.
func (e *Extractor) Extract(_ context.Context, src io.Reader, _ domain.MimeHint) (*domain.Result, error) {
	bytes, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("passthrough: read: %w", err)
	}
	return &domain.Result{
		Markdown:   string(bytes),
		Metadata:   map[string]any{"engine": "passthrough"},
		Pages:      0,
		Confidence: 100,
	}, nil
}
