// Wraps react-pdf with the worker config + a tiny pager UI.
//
// We import pdfjs-dist only inside this client component because
// it expects browser globals (`globalThis.window`, etc.). The
// dynamic import in DocumentReader keeps it out of the library
// list bundle.
"use client";

import { useState } from "react";
import { Document as PDFDocument, Page, pdfjs } from "react-pdf";

// Load the PDF.js worker from a CDN that mirrors the version on
// `pdfjs.version`. Avoiding `pdfjs-dist/build/pdf.worker.mjs` keeps
// the build pipeline simpler.
pdfjs.GlobalWorkerOptions.workerSrc = `https://cdn.jsdelivr.net/npm/pdfjs-dist@${pdfjs.version}/build/pdf.worker.min.mjs`;

interface PDFViewerProps {
  url: string;
}

export function PDFViewer({ url }: PDFViewerProps) {
  const [numPages, setNumPages] = useState<number | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const [error, setError] = useState<string | null>(null);

  if (error) {
    return (
      <p className="text-sm text-red-600 dark:text-red-400">Couldn&apos;t render PDF ({error}).</p>
    );
  }

  return (
    <div className="flex h-full flex-col items-center gap-4">
      <div className="flex-1 overflow-auto">
        <PDFDocument
          file={url}
          onLoadSuccess={({ numPages }) => {
            setNumPages(numPages);
            setError(null);
          }}
          onLoadError={(err) => setError(err.message)}
          loading={<div className="text-sm text-zinc-500">Loading PDF…</div>}
        >
          <Page
            pageNumber={pageNumber}
            width={520}
            renderAnnotationLayer={false}
            renderTextLayer={false}
          />
        </PDFDocument>
      </div>
      {numPages != null && (
        <div className="flex items-center gap-4 text-sm">
          <button
            type="button"
            onClick={() => setPageNumber((n) => Math.max(1, n - 1))}
            disabled={pageNumber <= 1}
            className="rounded-md border border-zinc-300 px-2 py-1 text-xs disabled:opacity-50 dark:border-zinc-700"
          >
            Prev
          </button>
          <span className="text-xs text-zinc-600 dark:text-zinc-400">
            Page {pageNumber} of {numPages}
          </span>
          <button
            type="button"
            onClick={() => setPageNumber((n) => (numPages != null ? Math.min(numPages, n + 1) : n))}
            disabled={pageNumber >= numPages}
            className="rounded-md border border-zinc-300 px-2 py-1 text-xs disabled:opacity-50 dark:border-zinc-700"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}
