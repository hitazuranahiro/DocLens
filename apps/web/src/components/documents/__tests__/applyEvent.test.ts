// Reducer tests for applyEvent (DocumentListLive).
//
// Live updates flow through this pure function before they touch
// state, so this is the right level to pin the contract: existing
// rows patch in place, new INSERTs prepend a placeholder, deletes
// remove, and unrelated UPDATEs are ignored.

import { describe, expect, it } from "vitest";

import type { components } from "@doclens/api-client";
import { applyEvent } from "../DocumentListLive";

type Document = NonNullable<components["schemas"]["Document"]>;
type DocumentStatusEvent = NonNullable<components["schemas"]["DocumentStatusEvent"]>;

const baseDoc = (overrides: Partial<Document> = {}): Document => ({
  id: "00000000-0000-0000-0000-000000000001",
  ownerId: "user_test",
  title: "Sample.pdf",
  sourceFilename: "Sample.pdf",
  sha256: "a".repeat(64),
  mimeType: "application/pdf",
  status: "queued",
  byteSize: 1024,
  pageCount: null,
  wordCount: null,
  confidence: null,
  lastError: null,
  createdAt: "2026-06-01T10:00:00Z",
  updatedAt: "2026-06-01T10:00:00Z",
  ...overrides,
});

const baseEvent = (overrides: Partial<DocumentStatusEvent> = {}): DocumentStatusEvent => ({
  documentId: "00000000-0000-0000-0000-000000000001",
  status: "extracting",
  updatedAt: "2026-06-01T10:01:00Z",
  ...overrides,
});

describe("applyEvent", () => {
  it("patches an existing document in place", () => {
    const doc = baseDoc();
    const ev = baseEvent({ status: "ready", pageCount: 12, wordCount: 3400, confidence: 95 });
    const next = applyEvent([doc], ev);
    expect(next).toHaveLength(1);
    expect(next[0]).toMatchObject({
      id: doc.id,
      status: "ready",
      pageCount: 12,
      wordCount: 3400,
      confidence: 95,
      updatedAt: ev.updatedAt,
    });
    // Untouched fields are preserved.
    expect(next[0]?.title).toBe(doc.title);
    expect(next[0]?.sourceFilename).toBe(doc.sourceFilename);
  });

  it("preserves existing metric values when an event omits them", () => {
    const doc = baseDoc({ pageCount: 5, wordCount: 1000, confidence: 80 });
    const ev = baseEvent({ status: "ready" });
    const next = applyEvent([doc], ev);
    expect(next[0]).toMatchObject({
      pageCount: 5,
      wordCount: 1000,
      confidence: 80,
      status: "ready",
    });
  });

  it("prepends a placeholder row for an INSERT it has not seen", () => {
    const existing = baseDoc({ id: "11111111-1111-1111-1111-111111111111" });
    const ev = baseEvent({
      documentId: "22222222-2222-2222-2222-222222222222",
      status: "queued",
      event: "INSERT",
    });
    const next = applyEvent([existing], ev);
    expect(next).toHaveLength(2);
    expect(next[0]?.id).toBe(ev.documentId);
    expect(next[0]?.title).toBe("Processing…");
    expect(next[1]?.id).toBe(existing.id);
  });

  it("ignores UPDATE for a document not in the current page", () => {
    const existing = baseDoc({ id: "11111111-1111-1111-1111-111111111111" });
    const ev = baseEvent({
      documentId: "22222222-2222-2222-2222-222222222222",
      status: "ready",
      event: "UPDATE",
    });
    const next = applyEvent([existing], ev);
    expect(next).toEqual([existing]);
  });

  it("removes a document on a deleted event", () => {
    const a = baseDoc({ id: "11111111-1111-1111-1111-111111111111" });
    const b = baseDoc({ id: "22222222-2222-2222-2222-222222222222" });
    const next = applyEvent(
      [a, b],
      baseEvent({
        documentId: a.id,
        status: "deleted",
      }),
    );
    expect(next).toEqual([b]);
  });

  it("captures last_error on a failed transition", () => {
    const doc = baseDoc({ status: "extracting" });
    const ev = baseEvent({ status: "failed", lastError: "extractor timed out" });
    const next = applyEvent([doc], ev);
    expect(next[0]).toMatchObject({ status: "failed", lastError: "extractor timed out" });
  });
});
