package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/tomeku/doclens/services/ingestion/domain"
)

var pdfOnly = map[string]struct{}{"application/pdf": {}}

func TestNewIntent_HappyPath(t *testing.T) {
	in, err := domain.NewIntent(
		"user_abc",
		"report.pdf",
		"application/pdf",
		1024,
		strings.Repeat("a", 64),
		"",
		pdfOnly,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Title != "report" {
		t.Fatalf("Title = %q, want %q (filename minus extension)", in.Title, "report")
	}
}

func TestNewIntent_RejectsOversize(t *testing.T) {
	_, err := domain.NewIntent(
		"u", "f.pdf", "application/pdf",
		domain.MaxUploadBytes+1,
		strings.Repeat("a", 64), "",
		pdfOnly,
	)
	if !errors.Is(err, domain.ErrUploadTooLarge) {
		t.Fatalf("err = %v, want ErrUploadTooLarge", err)
	}
}

func TestNewIntent_RejectsUnenabledMime(t *testing.T) {
	_, err := domain.NewIntent(
		"u", "f.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		1024, strings.Repeat("a", 64), "",
		pdfOnly,
	)
	if !errors.Is(err, domain.ErrUnsupportedMime) {
		t.Fatalf("err = %v, want ErrUnsupportedMime", err)
	}
}

func TestNewIntent_RejectsBadSHA(t *testing.T) {
	cases := map[string]string{
		"too short":    strings.Repeat("a", 63),
		"too long":     strings.Repeat("a", 65),
		"non-hex":      strings.Repeat("g", 64),
		"uppercase ok": strings.ToUpper(strings.Repeat("a", 64)), // normalized in
	}
	for name, hash := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := domain.NewIntent(
				"u", "f.pdf", "application/pdf",
				1024, hash, "", pdfOnly,
			)
			if name == "uppercase ok" {
				if err != nil {
					t.Fatalf("uppercase should normalize, got %v", err)
				}
				return
			}
			if !errors.Is(err, domain.ErrInvalidIntent) {
				t.Fatalf("err = %v, want ErrInvalidIntent", err)
			}
		})
	}
}

func TestNewIntent_TitleFallback(t *testing.T) {
	in, err := domain.NewIntent(
		"u", "Annual.Report.pdf", "application/pdf",
		1024, strings.Repeat("a", 64),
		"  Custom Title  ",
		pdfOnly,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in.Title != "Custom Title" {
		t.Fatalf("Title = %q, want trimmed custom title", in.Title)
	}

	in2, err := domain.NewIntent(
		"u", "Annual.Report.pdf", "application/pdf",
		1024, strings.Repeat("a", 64),
		"",
		pdfOnly,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if in2.Title != "Annual.Report" {
		t.Fatalf("Title fallback = %q, want filename without extension", in2.Title)
	}
}
