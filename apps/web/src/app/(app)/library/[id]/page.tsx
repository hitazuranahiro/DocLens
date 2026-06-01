// Reader page: shows the original PDF and the extracted Markdown
// side by side. For non-ready documents, renders a status-appropriate
// view (queued / extracting / failed) instead.

import Link from "next/link";

import { apiFromServer } from "@/lib/api";
import { DocumentReader } from "@/components/documents/DocumentReader";
import { StatusBadge } from "@/components/documents/StatusBadge";

export const dynamic = "force-dynamic";

export default async function DocumentReaderPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const client = await apiFromServer();

  const detail = await client.GET("/v1/documents/{id}", {
    params: { path: { id } },
  });
  if (detail.error || !detail.data) {
    return <NotFoundState id={id} status={detail.response?.status} />;
  }
  const doc = detail.data.document;

  if (doc.status !== "ready") {
    return <PendingState doc={doc} />;
  }

  // Get a presigned URL for the original PDF. The 5-minute TTL is
  // refreshed on every page load (Req 7.8).
  const raw = await client.GET("/v1/documents/{id}/raw", {
    params: { path: { id } },
  });
  if (raw.error || !raw.data) {
    return <ErrorState message="Couldn't sign the original file URL." />;
  }

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <div className="space-y-1">
          <Link
            href="/library"
            className="text-xs text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
          >
            ← Back to library
          </Link>
          <div className="flex items-center gap-2">
            <h1 className="text-xl font-semibold tracking-tight">{doc.title}</h1>
            <StatusBadge status={doc.status} />
          </div>
          <p className="text-xs text-zinc-500 dark:text-zinc-400">
            {doc.sourceFilename}
            {doc.pageCount != null && ` · ${doc.pageCount} pages`}
            {doc.wordCount != null && ` · ${doc.wordCount.toLocaleString()} words`}
            {doc.confidence != null && ` · confidence ${Math.round(doc.confidence)}`}
          </p>
        </div>
      </header>

      <DocumentReader
        doc={doc}
        pdfUrl={raw.data.url}
        markdownUrl={`/api/documents/${doc.id}/markdown`}
      />
    </div>
  );
}

// --- alternate states ----------------------------------------------------

function NotFoundState({ id, status }: { id: string; status: number | undefined }) {
  return (
    <div className="space-y-4">
      <Link
        href="/library"
        className="text-xs text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
      >
        ← Back to library
      </Link>
      <div className="rounded-lg border border-zinc-200 p-12 text-center dark:border-zinc-800">
        <p className="text-base font-medium">Document not found</p>
        <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-300">
          {status === 404 || status === undefined
            ? `No document with id ${id} in your library.`
            : `The API returned ${status}.`}
        </p>
      </div>
    </div>
  );
}

function PendingState({
  doc,
}: {
  doc: NonNullable<
    Awaited<ReturnType<Awaited<ReturnType<typeof apiFromServer>>["GET"]>>["data"]
  >["document"];
}) {
  const message =
    doc.status === "queued"
      ? "Waiting for an extraction worker to pick this up."
      : doc.status === "extracting"
        ? "Extraction is running. This page refreshes every few seconds."
        : doc.status === "failed"
          ? (doc.lastError ?? "Extraction failed without a recorded reason.")
          : "This document was deleted.";

  return (
    <div className="space-y-6">
      <Link
        href="/library"
        className="text-xs text-zinc-500 hover:text-zinc-900 dark:text-zinc-400 dark:hover:text-zinc-100"
      >
        ← Back to library
      </Link>
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <h1 className="text-xl font-semibold tracking-tight">{doc.title}</h1>
          <StatusBadge status={doc.status} />
        </div>
        <p className="text-sm text-zinc-600 dark:text-zinc-300">{message}</p>
        {doc.status === "failed" && (
          <p className="text-xs text-zinc-500 dark:text-zinc-400">
            Use the retry endpoint to re-enqueue the document. (UI button lands with M6 live
            status.)
          </p>
        )}
      </div>
    </div>
  );
}

function ErrorState({ message }: { message: string }) {
  return (
    <div className="rounded-lg border border-red-200 bg-red-50 p-6 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/40 dark:text-red-300">
      {message}
    </div>
  );
}
