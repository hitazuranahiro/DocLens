"use client";

import { useEffect, useState } from "react";
import dynamic from "next/dynamic";

import "react-pdf/dist/esm/Page/AnnotationLayer.css";
import "react-pdf/dist/esm/Page/TextLayer.css";

import type { components } from "@doclens/api-client";

type Document = NonNullable<components["schemas"]["Document"]>;

interface DocumentReaderProps {
  doc: Document;
  pdfUrl: string;
  markdownUrl: string;
}

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
    <section className="flex min-h-0 flex-col overflow-hidden rounded-md border border-border bg-surface">
      <header className="border-b border-border px-3 py-2 text-label uppercase tracking-wide text-muted">
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
    return <p className="text-body text-error">Couldn&apos;t load Markdown ({error}).</p>;
  }
  if (body == null) {
    return <PaneSpinner label="Loading Markdown…" />;
  }
  return (
    <pre
      className="min-h-full whitespace-pre-wrap break-words font-mono text-body text-text-strong"
      aria-label={`Markdown extraction of ${title}`}
    >
      {body}
    </pre>
  );
}

export function PaneSpinner({ label }: { label: string }) {
  return (
    <div className="flex h-full items-center justify-center text-caption text-muted">{label}</div>
  );
}
