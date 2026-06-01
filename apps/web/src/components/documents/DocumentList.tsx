// List view used by /library. Pure presentation: receives an array
// of documents from the page-level Server Component.

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
            className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-900"
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
    <div className="rounded-lg border border-dashed border-zinc-300 p-12 text-center dark:border-zinc-700">
      <p className="text-base font-medium">Your library is empty</p>
      <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-300">Upload a PDF to get started.</p>
      <Link
        href="/upload"
        className="mt-4 inline-flex rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-200"
      >
        Upload a document
      </Link>
    </div>
  );
}
