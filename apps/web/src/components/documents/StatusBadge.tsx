// Status pill that mirrors the Library document status enum.
// Pure presentation, no data fetching.

import type { components } from "@doclens/api-client";

type Status = NonNullable<components["schemas"]["Document"]>["status"];

interface StatusBadgeProps {
  status: Status;
}

const styles: Record<Status, { label: string; cls: string }> = {
  queued: {
    label: "Queued",
    cls: "bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300",
  },
  extracting: {
    label: "Extracting",
    cls: "bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-300",
  },
  ready: {
    label: "Ready",
    cls: "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300",
  },
  failed: {
    label: "Failed",
    cls: "bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-300",
  },
  deleted: {
    label: "Deleted",
    cls: "bg-zinc-100 text-zinc-500 dark:bg-zinc-900 dark:text-zinc-500",
  },
};

export function StatusBadge({ status }: StatusBadgeProps) {
  const s = styles[status];
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${s.cls}`}
    >
      {s.label}
    </span>
  );
}
