package app_test

import (
	"strings"
	"testing"

	"github.com/tomeku/doclens/services/extraction/app"
	"github.com/tomeku/doclens/services/extraction/domain"
)

func TestConfidence_EmptyMarkdownIsZero(t *testing.T) {
	if got := app.ConfidenceFor(&domain.Result{}); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
	if got := app.ConfidenceFor(nil); got != 0 {
		t.Fatalf("nil result got %d, want 0", got)
	}
}

func TestConfidence_NominalNoWarnings(t *testing.T) {
	r := &domain.Result{
		Markdown: "# Title\n\n" + strings.Repeat("word ", 250),
		Pages:    1,
	}
	got := app.ConfidenceFor(r)
	// No warnings → 90, density >= 200 → +10 = 100.
	if got != 100 {
		t.Fatalf("got %d, want 100", got)
	}
}

func TestConfidence_WarningsPenalize(t *testing.T) {
	r := &domain.Result{
		Markdown: strings.Repeat("a ", 100),
		Pages:    1,
		Warnings: []string{"missing font 1", "missing font 2", "missing font 3"},
	}
	got := app.ConfidenceFor(r)
	// 90 base - 15 (3 warns) +5 density (50<=100<200) = 80.
	if got != 80 {
		t.Fatalf("got %d, want 80", got)
	}
}

func TestConfidence_CriticalWarningHits(t *testing.T) {
	r := &domain.Result{
		Markdown: "# Doc",
		Pages:    1,
		Warnings: []string{"document is encrypted, falling back"},
	}
	got := app.ConfidenceFor(r)
	// 90 -5 (1 warn) -20 (critical) -10 (density<50) = 55.
	if got != 55 {
		t.Fatalf("got %d, want 55", got)
	}
}

func TestConfidence_PenaltyCapped(t *testing.T) {
	warns := make([]string, 50) // 50 warnings should cap at -30.
	for i := range warns {
		warns[i] = "noisy"
	}
	r := &domain.Result{
		Markdown: strings.Repeat("a ", 500),
		Pages:    1,
		Warnings: warns,
	}
	got := app.ConfidenceFor(r)
	// 90 -30 (capped) +10 density = 70.
	if got != 70 {
		t.Fatalf("got %d, want 70", got)
	}
}

func TestWordCount(t *testing.T) {
	cases := map[string]int{
		"":                 0,
		"hello":            1,
		"hello world":      2,
		"  hello   world ": 2,
		"a\nb\tc":          3,
	}
	for in, want := range cases {
		if got := app.WordCount(in); got != want {
			t.Errorf("WordCount(%q) = %d, want %d", in, got, want)
		}
	}
}
