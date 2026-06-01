// Library landing page.
//
// Server Component: fetches /v1/documents with the caller's Clerk
// token, renders the list, and links each row to the reader page.
// Cursor-based "Load more" is implemented as a plain link query so
// browsers without JS still paginate.

import Link from "next/link";

import { apiFromServer } from "@/lib/api";
import { DocumentList } from "@/components/documents/DocumentList";

export const dynamic = "force-dynamic";

export default async function LibraryPage({
  searchParams,
}: {
  searchParams: Promise<{ cursor?: string }>;
}) {
  const { cursor } = await searchParams;
  const client = await apiFromServer();
  const { data, error, response } = await client.GET("/v1/documents", {
    params: { query: cursor ? { cursor } : {} },
  });

  if (error || !data) {
    return <ErrorState status={response?.status} />;
  }

  const items = data.items ?? [];
  const nextHref = data.nextCursor
    ? `/library?cursor=${encodeURIComponent(data.nextCursor)}`
    : null;

  return (
    <div className="space-y-8">
      <header className="flex items-end justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold tracking-tight">Library</h1>
          <p className="text-sm text-zinc-600 dark:text-zinc-300">
            {items.length === 0
              ? "No documents yet."
              : data.nextCursor
                ? "Showing the most recent 20."
                : `${items.length} document${items.length === 1 ? "" : "s"}.`}
          </p>
        </div>
        <Link
          href="/upload"
          className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-zinc-800 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-200"
        >
          Upload
        </Link>
      </header>

      <DocumentList
        items={items}
        thumbnailHref={(doc) =>
          doc.status === "ready" ? `/api/documents/${doc.id}/thumbnail` : null
        }
        nextHref={nextHref}
      />
    </div>
  );
}

function ErrorState({ status }: { status: number | undefined }) {
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold tracking-tight">Library</h1>
      <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
        <p className="font-medium">Couldn&apos;t load your library</p>
        <p className="mt-1">
          The API returned {status ?? "no response"}. Try refreshing in a moment.
        </p>
      </div>
    </div>
  );
}
