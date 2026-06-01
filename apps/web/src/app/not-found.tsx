// Global 404. Plain server component without Clerk dependencies so
// prerendering doesn't require auth keys at build time.

import Link from "next/link";

export default function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-6 text-center">
      <p className="text-sm uppercase tracking-widest text-zinc-500">404</p>
      <h1 className="text-3xl font-semibold tracking-tight">Page not found</h1>
      <p className="max-w-md text-sm text-zinc-600 dark:text-zinc-300">
        The page you were looking for doesn&apos;t exist or was moved.
      </p>
      <Link
        href="/"
        className="rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium hover:bg-zinc-100 dark:border-zinc-700 dark:hover:bg-zinc-900"
      >
        Back to home
      </Link>
    </div>
  );
}
