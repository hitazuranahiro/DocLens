// Loading skeleton for /library. Next.js renders this while the
// page-level Server Component fetches /v1/documents.

export default function Loading() {
  return (
    <div className="space-y-8" aria-busy="true">
      <header className="space-y-2">
        <div className="h-7 w-32 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
        <div className="h-4 w-48 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
      </header>
      <ul className="space-y-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <li
            key={i}
            className="flex gap-4 rounded-lg border border-zinc-200 p-4 dark:border-zinc-800"
          >
            <div className="h-20 w-16 shrink-0 animate-pulse rounded-md bg-zinc-100 dark:bg-zinc-900" />
            <div className="min-w-0 flex-1 space-y-2">
              <div className="h-4 w-2/3 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
              <div className="h-3 w-1/3 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
              <div className="h-3 w-1/2 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
