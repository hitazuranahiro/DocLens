// Public landing page. Static, dependency-free server component.

import Link from "next/link";

export default function LandingPage() {
  return (
    <section className="mx-auto flex max-w-5xl flex-col gap-12 px-6 py-20">
      <div className="space-y-6">
        <p className="text-sm uppercase tracking-widest text-zinc-500">Open source · v0.1</p>
        <h1 className="text-4xl font-semibold tracking-tight md:text-6xl">
          Turn documents into knowledge.
        </h1>
        <p className="max-w-2xl text-lg text-zinc-600 dark:text-zinc-300">
          Upload a PDF. DocLens extracts a clean Markdown rendering, indexes it for full-text
          search, and shows you the original and the extraction side by side. Self-host it, own your
          data.
        </p>
        <div className="flex flex-wrap gap-3">
          <Link
            href="/sign-in"
            className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-200"
          >
            Get started
          </Link>
          <Link
            href="https://github.com/hitazuranahiro/DocLens"
            className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-900"
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
    <div className="rounded-lg border border-zinc-200 p-6 dark:border-zinc-800">
      <h2 className="text-base font-semibold">{title}</h2>
      <p className="mt-2 text-sm text-zinc-600 dark:text-zinc-300">{body}</p>
    </div>
  );
}
