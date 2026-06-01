// Skeleton for /library — mirrors the final layout so the page never
// reflows when data lands.

export default function Loading() {
  return (
    <div className="space-y-8" aria-busy="true">
      <header className="space-y-2">
        <div className="h-7 w-32 animate-pulse rounded-sm bg-surface" />
        <div className="h-4 w-48 animate-pulse rounded-sm bg-surface" />
      </header>
      <ul className="space-y-3">
        {Array.from({ length: 4 }).map((_, i) => (
          <li key={i} className="flex gap-4 rounded-md border border-border bg-surface p-4">
            <div className="h-20 w-16 shrink-0 animate-pulse rounded-md bg-bg" />
            <div className="min-w-0 flex-1 space-y-2">
              <div className="h-4 w-2/3 animate-pulse rounded-sm bg-bg" />
              <div className="h-3 w-1/3 animate-pulse rounded-sm bg-bg" />
              <div className="h-3 w-1/2 animate-pulse rounded-sm bg-bg" />
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
