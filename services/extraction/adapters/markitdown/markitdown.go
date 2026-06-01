// Package markitdown implements the Extractor port over the
// `markitdown` CLI from Microsoft.
//
// Behavior summary
//
//   - Input is written to a temp file (markitdown takes a path, not
//     a stream).
//   - We invoke `markitdown <path>` with the per-attempt timeout
//     wired into ctx.
//   - stdout is the Markdown body. stderr lines are collected as
//     warnings. A non-zero exit becomes ErrEngineCrashed unless ctx
//     was cancelled, in which case it becomes ErrTimeout.
//   - The Confidence score on the result is left at 0 here. The
//     scoring heuristic lives one layer up (in the worker) so we
//     can tune it without retesting the subprocess.
//
// The Runner interface lets unit tests substitute a fake without
// shelling out, and lets a future HTTP-sidecar mode plug in without
// touching the adapter.
package markitdown

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

// Runner runs `markitdown` over a file and returns its stdout/stderr.
//
// Implementations MUST honor ctx; the default exec runner passes ctx
// to exec.CommandContext so SIGKILL fires on cancellation.
type Runner interface {
	Run(ctx context.Context, path string) (stdout, stderr []byte, err error)
}

// Adapter wraps a Runner with the Extractor contract.
type Adapter struct {
	runner Runner
}

// Config bundles the adapter's options.
type Config struct {
	// Bin is the markitdown executable. Defaults to "markitdown" on PATH.
	Bin string
	// Runner overrides the default exec runner. Tests substitute a fake.
	Runner Runner
}

// New constructs an Adapter. If Runner is nil, the default exec
// runner is used with the configured binary path.
func New(cfg Config) *Adapter {
	r := cfg.Runner
	if r == nil {
		bin := cfg.Bin
		if bin == "" {
			bin = "markitdown"
		}
		r = &execRunner{bin: bin}
	}
	return &Adapter{runner: r}
}

// Extract implements domain.Extractor.
//
// We require the source to land on disk because markitdown takes a
// path. Streaming is a v0.2 concern; today's PDFs fit comfortably in
// /tmp.
func (a *Adapter) Extract(ctx context.Context, src io.Reader, hint domain.MimeHint) (*domain.Result, error) {
	tmpFile, err := writeTemp(src, hint.Filename)
	if err != nil {
		return nil, fmt.Errorf("markitdown: write temp: %w", err)
	}
	defer os.Remove(tmpFile)

	stdout, stderr, err := a.runner.Run(ctx, tmpFile)
	if err != nil {
		// Distinguish "ctx fired" from a real engine crash. The exec
		// runner wraps the os/exec error; we rely on ctx state rather
		// than parsing the exit code.
		if ctx.Err() != nil {
			return nil, domain.ErrTimeout
		}
		return nil, fmt.Errorf("%w: %v", domain.ErrEngineCrashed, err)
	}

	return &domain.Result{
		Markdown:   string(stdout),
		Warnings:   parseWarnings(stderr),
		Metadata:   map[string]any{"engine": "markitdown"},
		Pages:      0,
		Confidence: 0,
	}, nil
}

// writeTemp writes bytes to a uniquely named temp file. The extension
// is preserved when present because markitdown sniffs by path suffix.
func writeTemp(src io.Reader, filename string) (string, error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".bin"
	}
	f, err := os.CreateTemp("", "doclens-extract-*"+ext)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, src); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// parseWarnings turns stderr into a deduped, trimmed list of strings
// suitable for storage and display. We skip blank lines and strip
// common log-level prefixes that markitdown's deps emit.
func parseWarnings(stderr []byte) []string {
	if len(bytes.TrimSpace(stderr)) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	for _, raw := range bytes.Split(stderr, []byte("\n")) {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}
		// Strip "WARNING:", "ERROR:", etc. so the UI line doesn't
		// shout. The level is in the words that follow.
		for _, p := range []string{"WARNING:", "ERROR:", "INFO:", "DEBUG:"} {
			if strings.HasPrefix(line, p) {
				line = strings.TrimSpace(line[len(p):])
				break
			}
		}
		if _, dup := seen[line]; dup {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// execRunner is the production Runner.
type execRunner struct {
	bin string
}

// Run executes `<bin> <path>` and returns its stdout/stderr.
func (r *execRunner) Run(ctx context.Context, path string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, r.bin, path)

	// Buffered stdout/stderr. Capping is the caller's responsibility
	// — typical extractions produce well under 10 MB.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Surface stderr in the error so logs make sense without a
		// separate log line.
		return stdout.Bytes(), stderr.Bytes(), wrapExecErr(err, stderr.Bytes())
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func wrapExecErr(err error, stderr []byte) error {
	tail := tailString(string(stderr), 256)
	if tail == "" {
		return err
	}
	return fmt.Errorf("%w (stderr tail: %s)", err, tail)
}

func tailString(s string, n int) string {
	if len(s) <= n {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[len(s)-n:])
}

// Compile-time guarantees.
var (
	_ domain.Extractor = (*Adapter)(nil)
	_ error            = errors.New("")
)
