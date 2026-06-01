// HTML sanitization for search snippets.
//
// Postgres' ts_headline returns the body text with `<mark>` wrappers
// around matching terms. The body itself was extracted Markdown,
// which can contain anything the source PDF said; while ts_headline
// applies `StartSel`/`StopSel` literally rather than HTML-encoding
// surrounding text, the underlying body is still untrusted and
// could carry stray angle brackets from the original document.
//
// DOMPurify with a tight allowlist (`<mark>` only) gives us
// defense in depth without re-implementing an HTML parser.
//
// This module is browser-only — DOMPurify needs `window`. The only
// caller (SearchClient) is a "use client" component, so this is
// safe by construction. The defensive SSR fallback below keeps the
// module importable from server code at the cost of a bigger but
// equally safe escape (we strip everything when there's no DOM).

import DOMPurify from "dompurify";

/**
 * Returns a sanitized HTML fragment safe to assign to
 * `dangerouslySetInnerHTML`. Allows only `<mark>` tags; everything
 * else is stripped (textContent is preserved).
 */
export function sanitizeSnippet(raw: string): string {
  if (!raw) return "";
  if (typeof window === "undefined") {
    // No DOM available: fall back to a plain-text escape. We lose
    // the highlight markup but the result is unambiguously safe.
    return raw.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  }
  return DOMPurify.sanitize(raw, {
    ALLOWED_TAGS: ["mark"],
    ALLOWED_ATTR: [],
    KEEP_CONTENT: true,
  });
}
