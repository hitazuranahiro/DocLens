// Package pdftoppm implements the Thumbnailer port using the
// `pdftoppm` binary from Poppler.
//
// We render page 1 to a 256-pixel-wide PNG. PNG is chosen over WebP
// because pdftoppm doesn't support WebP natively and we don't want a
// second binary on the path just for thumbnails. WebP conversion can
// happen on the client (sharp / Next/Image) without changing the
// adapter.
package pdftoppm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tomeku/doclens/services/extraction/domain"
)

// Adapter shells out to the `pdftoppm` binary.
type Adapter struct {
	bin string
	// Width is the rendered width in pixels. Height scales to fit.
	width int
}

// Config bundles the adapter knobs.
type Config struct {
	// Bin overrides the executable path. Defaults to "pdftoppm".
	Bin string
	// Width sets the output width in pixels. Defaults to 256.
	Width int
}

// New returns an Adapter. If the binary is not on PATH, calls to
// Thumbnail will return ErrUnsupportedThumbnail rather than crashing.
func New(cfg Config) *Adapter {
	bin := cfg.Bin
	if bin == "" {
		bin = "pdftoppm"
	}
	width := cfg.Width
	if width <= 0 {
		width = 256
	}
	return &Adapter{bin: bin, width: width}
}

// Thumbnail implements domain.Thumbnailer.
//
// Logic
//   1. Verify the binary is on PATH; if not, signal "skip" cleanly.
//   2. Refuse non-PDF inputs by signaling "skip" too — workers
//      treat this as best-effort, not as a failed job.
//   3. Write the source to a temp file (pdftoppm takes a path).
//   4. Run `pdftoppm -png -f 1 -l 1 -scale-to-x <w> -scale-to-y -1 in.pdf out`.
//   5. Read out-1.png (pdftoppm appends "-1.png" to the prefix).
func (a *Adapter) Thumbnail(ctx context.Context, src io.Reader, hint domain.MimeHint) (*domain.Thumbnail, error) {
	if hint.MimeType != "" && hint.MimeType != "application/pdf" {
		return nil, domain.ErrUnsupportedThumbnail
	}
	if _, err := exec.LookPath(a.bin); err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrUnsupportedThumbnail, err)
	}

	tmpDir, err := os.MkdirTemp("", "doclens-thumb-*")
	if err != nil {
		return nil, fmt.Errorf("pdftoppm: mkdir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pdfPath := filepath.Join(tmpDir, "in.pdf")
	pdfFile, err := os.Create(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("pdftoppm: create input: %w", err)
	}
	if _, err := io.Copy(pdfFile, src); err != nil {
		pdfFile.Close()
		return nil, fmt.Errorf("pdftoppm: write input: %w", err)
	}
	pdfFile.Close()

	outPrefix := filepath.Join(tmpDir, "out")
	cmd := exec.CommandContext(ctx, a.bin,
		"-png",
		"-f", "1", "-l", "1",
		"-scale-to-x", fmt.Sprintf("%d", a.width),
		"-scale-to-y", "-1",
		pdfPath, outPrefix,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %v", domain.ErrUnsupportedThumbnail, ctx.Err())
		}
		// pdftoppm failures on encrypted/corrupt PDFs aren't
		// extraction failures. Skip cleanly.
		tail := strings.TrimSpace(stderr.String())
		return nil, fmt.Errorf("%w: %v (stderr: %s)", domain.ErrUnsupportedThumbnail, err, tail)
	}

	// pdftoppm names files <prefix>-<page>.png. With -f 1 -l 1 we
	// expect exactly one file; we list the directory to find it
	// since the page-number padding (e.g. "-1" vs "-01") varies
	// by version.
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("pdftoppm: list output: %w", err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(tmpDir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("pdftoppm: read output: %w", err)
		}
		return &domain.Thumbnail{Body: body, ContentType: "image/png"}, nil
	}
	return nil, errors.New("pdftoppm: no output png produced")
}

// Compile-time guarantee.
var _ domain.Thumbnailer = (*Adapter)(nil)
