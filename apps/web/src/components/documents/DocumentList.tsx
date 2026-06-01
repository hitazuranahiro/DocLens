// List view used by /library. Pure presentation.

import Link from "next/link";

import type { components } from "@doclens/api-client";
import { DocumentRow } from "./DocumentRow";

type Document = NonNullable<components["schemas"]["Document"]>;

interface DocumentListProps {
  items: Document[];
  /** Builds a URL for a document's thumbnail. Returns null when no thumbnail. */
  thumbnailHref: (doc: Document) => string | null;
  /** Page-2+ link, or null when there are no more pages. */
  nextHref: string | null;
}

export function DocumentList({ items, thumbnailHref, nextHref }: DocumentListProps) {
  if (items.length === 0) {
    return <EmptyState />;
  }
  return (
    <div className="space-y-3">
      <ul className="space-y-3">
        {items.map((doc) => (
          <li key={doc.id}>
            <DocumentRow doc={doc} thumbnailHref={thumbnailHref(doc)} />
          </li>
        ))}
      </ul>
      {nextHref && (
        <div className="flex justify-center pt-4">
          <Link
            href={nextHref}
            className="rounded-sm border border-border bg-surface px-4 py-2 text-label transition-colors duration-base hover:border-gray-400"
          >
            Load more
          </Link>
        </div>
      )}
    </div>
  );
}

function EmptyState() {
  return (
    <div className="rounded-lg border border-dashed border-border p-12 text-center">
      <p className="text-title text-text-strong">Your library is empty</p>
      <p className="mt-2 text-caption text-muted">Upload a PDF to get started.</p>
      <Link
        href="/upload"
        className="mt-4 inline-flex rounded-sm bg-brand px-4 py-2 text-label text-white transition-colors duration-base hover:opacity-90"
      >
        Upload a document
      </Link>
    </div>
  );
}
