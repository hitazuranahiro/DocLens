package markitdown_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tomeku/doclens/services/extraction/adapters/markitdown"
	"github.com/tomeku/doclens/services/extraction/domain"
)

type fakeRunner struct {
	stdout []byte
	stderr []byte
	err    error
}

func (f *fakeRunner) Run(_ context.Context, _ string) ([]byte, []byte, error) {
	return f.stdout, f.stderr, f.err
}

func TestExtract_HappyPath(t *testing.T) {
	a := markitdown.New(markitdown.Config{Runner: &fakeRunner{
		stdout: []byte("# Hello\n\nfrom MarkItDown."),
	}})
	res, err := a.Extract(context.Background(), strings.NewReader("ignored"), domain.MimeHint{
		MimeType: "application/pdf",
		Filename: "doc.pdf",
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !strings.HasPrefix(res.Markdown, "# Hello") {
		t.Fatalf("markdown = %q", res.Markdown)
	}
	if engine, _ := res.Metadata["engine"].(string); engine != "markitdown" {
		t.Fatalf("metadata.engine = %v, want markitdown", res.Metadata["engine"])
	}
}

func TestExtract_ParsesAndDedupesWarnings(t *testing.T) {
	a := markitdown.New(markitdown.Config{Runner: &fakeRunner{
		stdout: []byte("# Doc"),
		stderr: []byte("WARNING: missing font Helvetica\nWARNING: missing font Helvetica\nINFO: using fallback\n\n"),
	}})
	res, err := a.Extract(context.Background(), strings.NewReader(""), domain.MimeHint{Filename: "x.pdf"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := len(res.Warnings); got != 2 {
		t.Fatalf("warnings = %d, want 2 after dedupe (got %v)", got, res.Warnings)
	}
	if res.Warnings[0] != "missing font Helvetica" {
		t.Fatalf("warning[0] = %q", res.Warnings[0])
	}
	if res.Warnings[1] != "using fallback" {
		t.Fatalf("warning[1] = %q", res.Warnings[1])
	}
}

func TestExtract_ContextCancellationMapsToTimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	a := markitdown.New(markitdown.Config{Runner: &fakeRunner{err: errors.New("killed")}})
	_, err := a.Extract(ctx, strings.NewReader(""), domain.MimeHint{Filename: "x.pdf"})
	if !errors.Is(err, domain.ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}

func TestExtract_RunnerErrorMapsToCrashed(t *testing.T) {
	a := markitdown.New(markitdown.Config{Runner: &fakeRunner{err: errors.New("exit status 137")}})
	_, err := a.Extract(context.Background(), strings.NewReader(""), domain.MimeHint{Filename: "x.pdf"})
	if !errors.Is(err, domain.ErrEngineCrashed) {
		t.Fatalf("err = %v, want ErrEngineCrashed", err)
	}
}

// Smoke check that ctx is honored even if the runner returns nil.
func TestExtract_RespectsContextEvenOnSuccess(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	a := markitdown.New(markitdown.Config{Runner: &fakeRunner{stdout: []byte("# x")}})
	if _, err := a.Extract(ctx, strings.NewReader(""), domain.MimeHint{Filename: "x.pdf"}); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
