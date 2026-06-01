"use client";

import { useState } from "react";
import { Document as PDFDocument, Page, pdfjs } from "react-pdf";

pdfjs.GlobalWorkerOptions.workerSrc = `https://cdn.jsdelivr.net/npm/pdfjs-dist@${pdfjs.version}/build/pdf.worker.min.mjs`;

interface PDFViewerProps {
  url: string;
}

export function PDFViewer({ url }: PDFViewerProps) {
  const [numPages, setNumPages] = useState<number | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const [error, setError] = useState<string | null>(null);

  if (error) {
    return <p className="text-body text-error">Couldn&apos;t render PDF ({error}).</p>;
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
          loading={<div className="text-caption text-muted">Loading PDF…</div>}
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
        <div className="flex items-center gap-4">
          <button
            type="button"
            onClick={() => setPageNumber((n) => Math.max(1, n - 1))}
            disabled={pageNumber <= 1}
            className="rounded-sm border border-border bg-surface px-2 py-1 text-caption text-muted transition-colors duration-base hover:border-gray-400 disabled:opacity-50"
          >
            Prev
          </button>
          <span className="text-caption text-muted">
            Page {pageNumber} of {numPages}
          </span>
          <button
            type="button"
            onClick={() => setPageNumber((n) => (numPages != null ? Math.min(numPages, n + 1) : n))}
            disabled={pageNumber >= numPages}
            className="rounded-sm border border-border bg-surface px-2 py-1 text-caption text-muted transition-colors duration-base hover:border-gray-400 disabled:opacity-50"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}
