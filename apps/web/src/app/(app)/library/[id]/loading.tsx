// Skeleton for the document reader.

export default function Loading() {
  return (
    <div className="space-y-6" aria-busy="true">
      <div className="space-y-2">
        <div className="h-3 w-32 animate-pulse rounded-sm bg-surface" />
        <div className="h-7 w-1/2 animate-pulse rounded-sm bg-surface" />
      </div>
      <div className="grid h-[calc(100vh-12rem)] grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="rounded-md border border-border bg-surface p-4">
          <div className="h-full animate-pulse rounded-sm bg-bg" />
        </div>
        <div className="rounded-md border border-border bg-surface p-4">
          <div className="h-full animate-pulse rounded-sm bg-bg" />
        </div>
      </div>
    </div>
  );
}
