// Two-pane document reader.
//
// The PDF is rendered with react-pdf; we lazy-load it so the
// pdfjs-dist worker bundle doesn't bloat the library list bundle.
// The Markdown pane is plain <pre> for v0.1 — the spec calls for
// the user to be able to copy plain Markdown source (Req 4.5),
// so a rendered view is intentionally deferred.
"use client";

import { useEffect, useState } from "react";
import dynamic from "next/dynamic";

import "react-pdf/dist/esm/Page/AnnotationLayer.css";
import "react-pdf/dist/esm/Page/TextLayer.css";

import type { components } from "@doclens/api-client";

type Document = NonNullable<components["schemas"]["Document"]>;

interface DocumentReaderProps {
  doc: Document;
  /** URL to the raw PDF (presigned, server-fetched). */
  pdfUrl: string;
  /** URL to the proxied Markdown body (browser-side, auth-cookied). */
  markdownUrl: string;
}

// react-pdf imports pdfjs-dist which only works in the browser.
// next/dynamic with ssr:false is the canonical fix.
const PDFViewer = dynamic(() => import("./PDFViewer").then((m) => m.PDFViewer), {
  ssr: false,
  loading: () => <PaneSpinner label="Loading PDF…" />,
});

export function DocumentReader({ doc, pdfUrl, markdownUrl }: DocumentReaderProps) {
  return (
    <div className="grid h-[calc(100vh-12rem)] grid-cols-1 gap-4 lg:grid-cols-2">
      <Pane title="Original">
        <PDFViewer url={pdfUrl} />
      </Pane>
      <Pane title="Markdown">
        <MarkdownPane url={markdownUrl} title={doc.title} />
      </Pane>
    </div>
  );
}

function Pane({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-zinc-200 bg-zinc-50 dark:border-zinc-800 dark:bg-zinc-950">
      <header className="border-b border-zinc-200 px-3 py-2 text-xs font-medium uppercase tracking-wide text-zinc-600 dark:border-zinc-800 dark:text-zinc-400">
        {title}
      </header>
      <div className="flex-1 overflow-auto p-4">{children}</div>
    </section>
  );
}

function MarkdownPane({ url, title }: { url: string; title: string }) {
  const [body, setBody] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    fetch(url, { signal: controller.signal })
      .then(async (resp) => {
        if (!resp.ok) {
          throw new Error(`HTTP ${resp.status}`);
        }
        return resp.text();
      })
      .then(setBody)
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setError(err instanceof Error ? err.message : "failed");
      });
    return () => controller.abort();
  }, [url]);

  if (error) {
    return (
      <p className="text-sm text-red-600 dark:text-red-400">
        Couldn&apos;t load Markdown ({error}).
      </p>
    );
  }
  if (body == null) {
    return <PaneSpinner label="Loading Markdown…" />;
  }
  // <pre> preserves newlines and lets the user copy plain Markdown
  // source (Req 4.5). When we add a rendered toggle in v0.2, this
  // pane gets a sibling.
  return (
    <pre
      className="min-h-full whitespace-pre-wrap break-words font-mono text-sm text-zinc-800 dark:text-zinc-200"
      aria-label={`Markdown extraction of ${title}`}
    >
      {body}
    </pre>
  );
}

export function PaneSpinner({ label }: { label: string }) {
  return (
    <div className="flex h-full items-center justify-center text-sm text-zinc-500 dark:text-zinc-400">
      {label}
    </div>
  );
}
