// One row in the library list. Renders the thumbnail (or an emoji
// fallback), the title + filename, status badge, and a few metric
// chips for ready documents.

import Link from "next/link";
import Image from "next/image";

import type { components } from "@doclens/api-client";
import { StatusBadge } from "./StatusBadge";

type Document = NonNullable<components["schemas"]["Document"]>;

interface DocumentRowProps {
  doc: Document;
  /** URL to the page-1 thumbnail, or null if the document has none. */
  thumbnailHref: string | null;
}

export function DocumentRow({ doc, thumbnailHref }: DocumentRowProps) {
  return (
    <Link
      href={`/library/${doc.id}`}
      className="group flex items-start gap-4 rounded-lg border border-zinc-200 p-4 transition-colors hover:border-zinc-400 dark:border-zinc-800 dark:hover:border-zinc-600"
    >
      <Thumbnail href={thumbnailHref} title={doc.title} />
      <div className="min-w-0 flex-1 space-y-1.5">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="truncate text-base font-medium">{doc.title}</h3>
          <StatusBadge status={doc.status} />
        </div>
        <p className="truncate text-xs text-zinc-500 dark:text-zinc-400">
          {doc.sourceFilename} · {formatBytes(doc.byteSize)}
        </p>
        {doc.status === "ready" && (
          <div className="flex flex-wrap gap-3 pt-1 text-xs text-zinc-600 dark:text-zinc-400">
            {doc.pageCount != null && <span>{doc.pageCount} pages</span>}
            {doc.wordCount != null && <span>{doc.wordCount.toLocaleString()} words</span>}
            {doc.confidence != null && <span>Confidence {Math.round(doc.confidence)}</span>}
          </div>
        )}
        {doc.status === "failed" && doc.lastError && (
          <p className="truncate text-xs text-red-600 dark:text-red-400">{doc.lastError}</p>
        )}
      </div>
    </Link>
  );
}

function Thumbnail({ href, title }: { href: string | null; title: string }) {
  if (!href) {
    return (
      <div className="flex h-20 w-16 shrink-0 items-center justify-center rounded-md bg-zinc-100 text-2xl dark:bg-zinc-900">
        📄
      </div>
    );
  }
  // The API returns image/png with auth required; the browser will
  // include the Clerk session cookie via Next's middleware proxy.
  // We use unoptimized={true} so Next/Image doesn't try to fetch
  // through its loader (which doesn't carry auth headers).
  return (
    <div className="relative h-20 w-16 shrink-0 overflow-hidden rounded-md bg-zinc-100 dark:bg-zinc-900">
      <Image
        src={href}
        alt={`Page 1 of ${title}`}
        fill
        sizes="64px"
        className="object-cover"
        unoptimized
      />
    </div>
  );
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`;
  return `${(n / (1024 * 1024)).toFixed(n < 10 * 1024 * 1024 ? 1 : 0)} MB`;
}
