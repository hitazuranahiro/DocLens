// Loading skeleton for the document reader.

export default function Loading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <div className="space-y-2">
        <div className="h-3 w-32 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
        <div className="h-7 w-1/2 animate-pulse rounded bg-zinc-200 dark:bg-zinc-800" />
      </div>
      <div className="grid h-[calc(100vh-12rem)] grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-4 dark:border-zinc-800 dark:bg-zinc-950">
          <div className="h-full animate-pulse rounded bg-zinc-100 dark:bg-zinc-900" />
        </div>
        <div className="rounded-lg border border-zinc-200 bg-zinc-50 p-4 dark:border-zinc-800 dark:bg-zinc-950">
          <div className="h-full animate-pulse rounded bg-zinc-100 dark:bg-zinc-900" />
        </div>
      </div>
    </div>
  );
}
