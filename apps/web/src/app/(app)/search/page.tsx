// Search landing. Server Component shell + client island for the
// debounced input and result list.
//
// We keep the URL as the source of truth for `q` so the page is
// shareable and the back button works. The Server Component reads
// `?q=` and renders the initial result set (or an empty intro if
// no query is present).

import { apiFromServer } from "@/lib/api";
import { SearchClient } from "@/components/search/SearchClient";

export const dynamic = "force-dynamic";

export default async function SearchPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string; cursor?: string }>;
}) {
  const { q = "", cursor } = await searchParams;

  // Empty query → render the intro state without hitting the API.
  if (q.trim().length === 0) {
    return (
      <div className="space-y-8">
        <header className="space-y-2">
          <h1 className="text-heading text-text-strong">Search</h1>
          <p className="text-body text-muted">
            Find anything across your library. Quoted phrases and
            <code className="ml-1 mr-1 rounded bg-bg px-1 text-caption">-exclude</code>
            terms are supported.
          </p>
        </header>
        <SearchClient initialQuery="" initialResults={null} />
      </div>
    );
  }

  const client = await apiFromServer();
  const { data, error, response } = await client.GET("/v1/search", {
    params: { query: { q, ...(cursor ? { cursor } : {}) } },
  });

  if (error || !data) {
    return <ErrorState status={response?.status} q={q} />;
  }

  return (
    <div className="space-y-8">
      <header className="space-y-2">
        <h1 className="text-heading text-text-strong">Search</h1>
        <p className="text-body text-muted">
          {data.items.length === 0
            ? `No results for "${q}".`
            : `${data.items.length} result${data.items.length === 1 ? "" : "s"} for "${q}".`}
        </p>
      </header>
      <SearchClient
        initialQuery={q}
        initialResults={{ items: data.items, nextCursor: data.nextCursor ?? null }}
      />
    </div>
  );
}

function ErrorState({ status, q }: { status: number | undefined; q: string }) {
  return (
    <div className="space-y-4">
      <h1 className="text-heading text-text-strong">Search</h1>
      <div className="rounded-md border border-error bg-error-surface p-6 text-body text-error">
        <p className="font-medium">Couldn&apos;t complete search for &ldquo;{q}&rdquo;</p>
        <p className="mt-1">
          The API returned {status ?? "no response"}. Try refreshing in a moment.
        </p>
      </div>
    </div>
  );
}
