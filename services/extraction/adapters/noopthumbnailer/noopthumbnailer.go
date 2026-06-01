// Package noopthumbnailer is a Thumbnailer that always reports the
// input as unsupported. Used in tests and dev environments where
// Poppler is not installed.
package noopthumbnailer

import (
	"context"
	"io"

	"github.com/tomeku/doclens/services/extraction/domain"
)

// Adapter is the noop thumbnailer.
type Adapter struct{}

// New returns an Adapter.
func New() *Adapter { return &Adapter{} }

// Thumbnail always returns ErrUnsupportedThumbnail.
func (Adapter) Thumbnail(_ context.Context, _ io.Reader, _ domain.MimeHint) (*domain.Thumbnail, error) {
	return nil, domain.ErrUnsupportedThumbnail
}
