// Public landing page. Static, dependency-free server component.

import Link from "next/link";

export default function LandingPage() {
  return (
    <section className="mx-auto flex max-w-5xl flex-col gap-12 px-6 py-12">
      <div className="space-y-6">
        <p className="text-label uppercase tracking-widest text-muted">Open source · v0.1</p>
        <h1 className="text-display tracking-tight text-text-strong">
          Turn documents into knowledge.
        </h1>
        <p className="max-w-2xl text-body text-muted">
          Upload a PDF. DocLens extracts a clean Markdown rendering, indexes it for full-text
          search, and shows you the original and the extraction side by side. Self-host it, own your
          data.
        </p>
        <div className="flex flex-wrap gap-3">
          <Link
            href="/sign-in"
            className="rounded-sm bg-brand px-4 py-2 text-label text-white transition-opacity duration-base hover:opacity-90"
          >
            Get started
          </Link>
          <Link
            href="https://github.com/hitazuranahiro/DocLens"
            className="rounded-sm border border-border bg-surface px-4 py-2 text-label text-text-strong transition-colors duration-base hover:border-gray-400"
          >
            View on GitHub
          </Link>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <Feature
          title="Upload"
          body="Drag in a PDF. We presign the upload, dedupe on hash, and queue extraction."
        />
        <Feature title="Read" body="See the original and the Markdown extraction side by side." />
        <Feature
          title="Search"
          body="Full-text search across your library with snippets and quick jumps."
        />
      </div>
    </section>
  );
}

function Feature({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-md border border-border bg-surface p-6 shadow-md">
      <h2 className="text-title text-text-strong">{title}</h2>
      <p className="mt-2 text-caption text-muted">{body}</p>
    </div>
  );
}
