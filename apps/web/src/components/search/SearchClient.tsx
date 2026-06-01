// Client-side search island.
//
// Owns:
//   * the debounced query input
//   * the result list with sanitized snippets
//   * URL syncing (router.replace so the back button still works)
//
// Why a debounce instead of a typing-in-progress fetch? The API does
// real work (websearch_to_tsquery + ts_headline) on every call;
// firing on every keystroke adds nothing for the user but multiplies
// load. 250ms is the standard "felt instant" threshold.

"use client";

import { useEffect, useRef, useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";

import type { components } from "@doclens/api-client";
import { sanitizeSnippet } from "@/lib/sanitize";

type SearchHit = NonNullable<components["schemas"]["SearchHit"]>;

interface SearchResults {
  items: SearchHit[];
  nextCursor: string | null;
}

interface SearchClientProps {
  initialQuery: string;
  /** Server-rendered first page; null when the URL had no `q`. */
  initialResults: SearchResults | null;
}

const DEBOUNCE_MS = 250;

export function SearchClient({ initialQuery, initialResults }: SearchClientProps) {
  const router = useRouter();
  const [query, setQuery] = useState(initialQuery);
  const [, startTransition] = useTransition();
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Push the query to the URL after the debounce window. The Server
  // Component re-renders the result list — we don't fetch in the
  // client. This keeps server-rendering, prefetching, and bookmarks
  // working with zero extra plumbing.
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      const next = query.trim();
      const target = next ? `/search?q=${encodeURIComponent(next)}` : "/search";
      startTransition(() => router.replace(target));
    }, DEBOUNCE_MS);
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
    // initialQuery is a one-shot seed — only `query` needs to drive this.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query]);

  return (
    <div className="space-y-6">
      <input
        type="search"
        autoFocus
        autoComplete="off"
        spellCheck={false}
        aria-label="Search your library"
        placeholder='Try "deployment plan" or onboarding -draft'
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        className="w-full rounded-md border border-border bg-surface px-4 py-3 text-body text-text-strong placeholder:text-muted focus:border-brand focus:outline-none"
      />

      {initialResults === null ? (
        <EmptyIntro />
      ) : initialResults.items.length === 0 ? (
        <EmptyResults />
      ) : (
        <ResultList items={initialResults.items} />
      )}
    </div>
  );
}

function ResultList({ items }: { items: SearchHit[] }) {
  return (
    <ul className="space-y-3">
      {items.map((hit) => (
        <li key={hit.documentId}>
          <ResultRow hit={hit} />
        </li>
      ))}
    </ul>
  );
}

function ResultRow({ hit }: { hit: SearchHit }) {
  return (
    <Link
      href={`/library/${hit.documentId}`}
      className="block rounded-md border border-border bg-surface p-4 transition-colors duration-base hover:border-gray-400"
    >
      <div className="flex items-baseline justify-between gap-4">
        <h3 className="truncate text-title text-text-strong">{hit.title}</h3>
        <span className="shrink-0 text-caption text-muted">rank {hit.rank.toFixed(2)}</span>
      </div>
      <p
        className="mt-2 line-clamp-3 text-body text-muted [&_mark]:bg-brand-soft [&_mark]:px-0.5 [&_mark]:text-text-strong"
        // The API returns `<mark>`-wrapped snippets from ts_headline.
        // We sanitize defensively even though Postgres' StartSel is
        // a literal string (no user content reaches it).
        dangerouslySetInnerHTML={{ __html: sanitizeSnippet(hit.snippet) }}
      />
    </Link>
  );
}

function EmptyIntro() {
  return (
    <div className="rounded-lg border border-dashed border-border p-12 text-center">
      <p className="text-title text-text-strong">Type to search</p>
      <p className="mt-2 text-caption text-muted">Results appear instantly as you type.</p>
    </div>
  );
}

function EmptyResults() {
  return (
    <div className="rounded-lg border border-dashed border-border p-12 text-center">
      <p className="text-title text-text-strong">No matches</p>
      <p className="mt-2 text-caption text-muted">Try fewer words or remove negations.</p>
    </div>
  );
}
