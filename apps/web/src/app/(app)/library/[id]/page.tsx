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

  const raw = await client.GET("/v1/documents/{id}/raw", {
    params: { path: { id } },
  });
  if (raw.error || !raw.data) {
    return <ErrorState message="Couldn't sign the original file URL." />;
  }

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <Link href="/library" className="text-caption text-muted hover:text-text-strong">
          ← Back to library
        </Link>
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-title text-text-strong">{doc.title}</h1>
          <StatusBadge status={doc.status} />
        </div>
        <p className="text-caption text-muted">
          {doc.sourceFilename}
          {doc.pageCount != null && ` · ${doc.pageCount} pages`}
          {doc.wordCount != null && ` · ${doc.wordCount.toLocaleString()} words`}
          {doc.confidence != null && ` · confidence ${Math.round(doc.confidence)}`}
        </p>
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
    <div className="space-y-6">
      <Link href="/library" className="text-caption text-muted hover:text-text-strong">
        ← Back to library
      </Link>
      <div className="rounded-md border border-border bg-surface p-12 text-center">
        <p className="text-title text-text-strong">Document not found</p>
        <p className="mt-2 text-body text-muted">
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
      <Link href="/library" className="text-caption text-muted hover:text-text-strong">
        ← Back to library
      </Link>
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <h1 className="text-title text-text-strong">{doc.title}</h1>
          <StatusBadge status={doc.status} />
        </div>
        <p className="text-body text-muted">{message}</p>
        {doc.status === "failed" && (
          <p className="text-caption text-muted">
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
    <div className="rounded-md border border-error bg-error-surface p-6 text-body text-error">
      {message}
    </div>
  );
}
