// Client-side live wrapper around DocumentList.
//
// The Server Component fetches the initial page and passes it in as
// `initialItems`. We hold that list in state and patch it whenever
// the SSE feed delivers an update for one of those documents.
//
// New documents that appear in the feed (event="INSERT") are
// prepended to the list — this is what makes a freshly uploaded
// document show up without a reload.

"use client";

import { useCallback, useState } from "react";

import type { components } from "@doclens/api-client";
import { DocumentList } from "./DocumentList";
import { StreamIndicator } from "./StreamIndicator";
import { useDocumentStream, type DocumentStatusEvent } from "@/hooks/useDocumentStream";

type Document = NonNullable<components["schemas"]["Document"]>;

interface DocumentListLiveProps {
  initialItems: Document[];
  thumbnailHrefFor: (id: string) => string | null;
  /** Pagination link from the Server Component, opaque to live updates. */
  nextHref: string | null;
}

export function DocumentListLive({
  initialItems,
  thumbnailHrefFor,
  nextHref,
}: DocumentListLiveProps) {
  const [items, setItems] = useState<Document[]>(initialItems);

  const handleEvent = useCallback((ev: DocumentStatusEvent) => {
    setItems((prev) => applyEvent(prev, ev));
  }, []);

  const { state } = useDocumentStream(handleEvent);

  return (
    <div className="space-y-3">
      <div className="flex justify-end">
        <StreamIndicator state={state} />
      </div>
      <DocumentList
        items={items}
        thumbnailHref={(doc) => (doc.status === "ready" ? thumbnailHrefFor(doc.id) : null)}
        nextHref={nextHref}
      />
    </div>
  );
}

/**
 * Pure reducer. Exported for unit tests.
 *
 * - UPDATE for an in-list document patches it in place.
 * - UPDATE for a not-in-list document is ignored (must be on a later page).
 * - INSERT for a new document prepends a placeholder row.
 * - status="deleted" removes the document.
 */
export function applyEvent(items: Document[], ev: DocumentStatusEvent): Document[] {
  if (ev.status === "deleted") {
    return items.filter((d) => d.id !== ev.documentId);
  }

  const idx = items.findIndex((d) => d.id === ev.documentId);
  if (idx === -1) {
    if (ev.event === "INSERT") {
      return [eventToPlaceholder(ev), ...items];
    }
    return items;
  }
  const next = items.slice();
  next[idx] = mergeEvent(items[idx]!, ev);
  return next;
}

function mergeEvent(doc: Document, ev: DocumentStatusEvent): Document {
  return {
    ...doc,
    status: ev.status,
    pageCount: ev.pageCount ?? doc.pageCount,
    wordCount: ev.wordCount ?? doc.wordCount,
    confidence: ev.confidence ?? doc.confidence,
    lastError: ev.lastError ?? doc.lastError,
    updatedAt: ev.updatedAt,
  };
}

/**
 * Build a placeholder Document row for an INSERT event we haven't
 * fetched yet. The list keeps it visible until the next refresh
 * upgrades it with the real metadata; the title is intentionally
 * generic because the trigger payload doesn't include source_filename
 * or title (those wouldn't fit under the 8KB notify cap and are
 * trivial to refetch).
 */
function eventToPlaceholder(ev: DocumentStatusEvent): Document {
  return {
    id: ev.documentId,
    ownerId: "", // not exposed, filled in on next list refresh
    title: "Processing…",
    sourceFilename: "—",
    sha256: "",
    mimeType: "",
    status: ev.status,
    byteSize: 0,
    pageCount: ev.pageCount ?? null,
    wordCount: ev.wordCount ?? null,
    confidence: ev.confidence ?? null,
    lastError: ev.lastError ?? null,
    createdAt: ev.updatedAt,
    updatedAt: ev.updatedAt,
  };
}
