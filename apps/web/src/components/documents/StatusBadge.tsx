// Status pill that mirrors the Library document status enum.
// Pure presentation, no data fetching.
//
// Tokens come from DESIGN.md: surface = semantic color @ 12% opacity,
// text = semantic color @ 100%. Pill shape uses radius-full.

import type { components } from "@doclens/api-client";

type Status = NonNullable<components["schemas"]["Document"]>["status"];

interface StatusBadgeProps {
  status: Status;
}

const styles: Record<Status, { label: string; cls: string }> = {
  queued: {
    label: "Queued",
    cls: "bg-surface text-muted",
  },
  extracting: {
    label: "Extracting",
    cls: "bg-info-surface text-info",
  },
  ready: {
    label: "Ready",
    cls: "bg-success-surface text-success",
  },
  failed: {
    label: "Failed",
    cls: "bg-error-surface text-error",
  },
  deleted: {
    label: "Deleted",
    cls: "bg-surface text-muted",
  },
};

export function StatusBadge({ status }: StatusBadgeProps) {
  const s = styles[status];
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-1 text-label font-medium ${s.cls}`}
    >
      {s.label}
    </span>
  );
}
