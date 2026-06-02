import Link from "next/link";

import { apiFromServer } from "@/lib/api";
import { DocumentListLive } from "@/components/documents/DocumentListLive";

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
        <div className="space-y-2">
          <h1 className="text-heading text-text-strong">Library</h1>
          <p className="text-body text-muted">
            {items.length === 0
              ? "No documents yet."
              : data.nextCursor
                ? "Showing the most recent 20."
                : `${items.length} document${items.length === 1 ? "" : "s"}.`}
          </p>
        </div>
        <Link
          href="/upload"
          className="rounded-sm bg-brand px-3 py-1.5 text-label text-white transition-opacity duration-base hover:opacity-90"
        >
          Upload
        </Link>
      </header>

      <DocumentListLive
        initialItems={items}
        thumbnailHrefPrefix="/api/documents/"
        nextHref={nextHref}
      />
    </div>
  );
}

function ErrorState({ status }: { status: number | undefined }) {
  return (
    <div className="space-y-4">
      <h1 className="text-heading text-text-strong">Library</h1>
      <div className="rounded-md border border-error bg-error-surface p-6 text-body text-error">
        <p className="font-medium">Couldn&apos;t load your library</p>
        <p className="mt-1">
          The API returned {status ?? "no response"}. Try refreshing in a moment.
        </p>
      </div>
    </div>
  );
}
