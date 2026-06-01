// Package app holds the extraction worker use case.
package app

import (
	"strings"
	"unicode"

	"github.com/tomeku/doclens/services/extraction/domain"
)

// ConfidenceFor scores an extraction Result on a 0..100 scale.
//
// Heuristic (deliberately simple — easier to tune than ML):
//
//   - Start at 90 (we never give 100; nothing is perfect).
//   - Subtract 5 per warning, capped at -30.
//   - If the worker produced no Markdown, score is 0.
//   - If we have a page count and word count, reward word density:
//     >=200 words/page → +10, 50–200 → +5, <50 → -10.
//   - If known critical signals appear in warnings, force a penalty:
//     "encrypted", "corrupt", "ocr required" → -20.
//   - Clamp to [0,100].
//
// The scoring lives in extraction/app rather than in the MarkItDown
// adapter so it stays adapter-agnostic. A future Poppler adapter
// reuses the same scorer.
func ConfidenceFor(r *domain.Result) int {
	if r == nil || strings.TrimSpace(r.Markdown) == "" {
		return 0
	}

	score := 90

	warnPenalty := 5 * len(r.Warnings)
	if warnPenalty > 30 {
		warnPenalty = 30
	}
	score -= warnPenalty

	for _, w := range r.Warnings {
		lw := strings.ToLower(w)
		if strings.Contains(lw, "encrypted") ||
			strings.Contains(lw, "corrupt") ||
			strings.Contains(lw, "ocr required") {
			score -= 20
			break // single critical penalty
		}
	}

	if r.Pages > 0 {
		words := WordCount(r.Markdown)
		density := float64(words) / float64(r.Pages)
		switch {
		case density >= 200:
			score += 10
		case density >= 50:
			score += 5
		case density < 50:
			score -= 10
		}
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// WordCount counts whitespace-separated runs of non-space runes in s.
// Cheap and good enough for ranking; we don't need linguistic accuracy.
func WordCount(s string) int {
	count := 0
	inWord := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if inWord {
				count++
				inWord = false
			}
		} else {
			inWord = true
		}
	}
	if inWord {
		count++
	}
	return count
}
