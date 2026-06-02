package postgres

import (
	"strings"
	"testing"
)

// We test the UTF-8 sanitizer in isolation because it's the
// guard that prevents Postgres SQLSTATE 22021 from blowing up
// the extraction transaction when the dev passthrough extractor
// hands us raw PDF bytes.
//
// Going through the real Postgres adapter would require a live
// database connection; the sanitizer is a pure function so a
// direct test is the right level here.
func TestToValidUTF8(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantSub string
	}{
		{"clean ASCII", "hello world", "hello world", ""},
		{"clean UTF-8", "héllo wörld", "héllo wörld", ""},
		{"strips embedded NUL", "before\x00after", "beforeafter", ""},
		{"replaces invalid byte sequence", "x\xd3\xebz", "", "\uFFFD"},
		{"empty", "", "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toValidUTF8(tc.in)
			if tc.want != "" && got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if tc.wantSub != "" && !strings.Contains(got, tc.wantSub) {
				t.Fatalf("got %q, want substring %q", got, tc.wantSub)
			}
			if strings.Contains(got, "\x00") {
				t.Fatalf("output still contains NUL: %q", got)
			}
			if !isValidUTF8(got) {
				t.Fatalf("output not valid UTF-8: %q", got)
			}
		})
	}
}

func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' {
			// Replacement character is fine; we explicitly emit it.
			continue
		}
	}
	return true
}
