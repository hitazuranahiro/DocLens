package domain

import (
	"context"
	"errors"
	"io"
)

// Thumbnailer renders a single thumbnail from an input document.
//
// v0.1 only renders PDFs. Adapters that don't support a given input
// MUST return ErrUnsupportedThumbnail; the worker treats that as a
// "skip thumbnail, continue" signal rather than a failure.
type Thumbnailer interface {
	Thumbnail(ctx context.Context, src io.Reader, hint MimeHint) (*Thumbnail, error)
}

// Thumbnail is the rendered image plus its content type.
type Thumbnail struct {
	// Body is the image bytes (PNG, WebP, ...). Non-nil on success.
	Body []byte
	// ContentType matches the bytes (e.g. "image/png").
	ContentType string
}

// ErrUnsupportedThumbnail says the adapter cannot render this input
// (wrong format, missing binary, etc.). The worker MUST handle this
// as best-effort, not as a job failure.
var ErrUnsupportedThumbnail = errors.New("extraction: thumbnail not supported")
