// Global 404. Plain server component without Clerk dependencies so
// prerendering doesn't require auth keys at build time.

import Link from "next/link";

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 bg-bg px-6 text-center">
      <p className="text-label uppercase tracking-widest text-muted">404</p>
      <h1 className="text-heading text-text-strong">Page not found</h1>
      <p className="max-w-md text-body text-muted">
        The page you were looking for doesn&apos;t exist or was moved.
      </p>
      <Link
        href="/"
        className="rounded-sm border border-border bg-surface px-4 py-2 text-label text-text-strong transition-colors duration-base hover:border-gray-400"
      >
        Back to home
      </Link>
    </div>
  );
}
