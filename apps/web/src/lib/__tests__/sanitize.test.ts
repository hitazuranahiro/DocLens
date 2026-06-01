// sanitizeSnippet contract: <mark> survives, everything else is
// stripped. Runs in the jsdom test environment so the browser
// DOMPurify branch is exercised.

import { describe, expect, it } from "vitest";

import { sanitizeSnippet } from "../sanitize";

describe("sanitizeSnippet", () => {
  it("returns empty string for empty input", () => {
    expect(sanitizeSnippet("")).toBe("");
  });

  it("preserves <mark> tags from ts_headline", () => {
    const got = sanitizeSnippet("hello <mark>world</mark>");
    expect(got).toContain("<mark>world</mark>");
    expect(got).toContain("hello");
  });

  it("strips <script> tags but preserves their textContent", () => {
    const got = sanitizeSnippet("safe <script>alert(1)</script> text");
    expect(got).not.toContain("<script>");
    // KEEP_CONTENT: true keeps the inner text.
    expect(got).toContain("safe");
    expect(got).toContain("text");
  });

  it("strips disallowed attributes from <mark>", () => {
    const got = sanitizeSnippet('<mark onclick="alert(1)">x</mark>');
    expect(got).toBe("<mark>x</mark>");
  });

  it("strips <img> tags including event handlers", () => {
    const got = sanitizeSnippet('<img src=x onerror="alert(1)">');
    expect(got).not.toContain("<img");
    expect(got).not.toContain("onerror");
  });

  it("preserves multiple marks in a fragment", () => {
    const got = sanitizeSnippet("the <mark>quick</mark> brown <mark>fox</mark> jumps");
    expect(got).toContain("<mark>quick</mark>");
    expect(got).toContain("<mark>fox</mark>");
  });
});
