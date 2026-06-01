// Tiny status pill that surfaces the SSE connection state.
//
// We keep it visible only when the connection is in motion or
// degraded — a green "live" dot would just add noise once the user
// has glanced at it, so we render the open state as a subtle
// pulsing dot in the corner without the word "live".

"use client";

import type { StreamConnectionState } from "@/hooks/useDocumentStream";

interface StreamIndicatorProps {
  state: StreamConnectionState;
}

export function StreamIndicator({ state }: StreamIndicatorProps) {
  if (state === "open") {
    return (
      <span className="inline-flex items-center gap-2 text-caption text-muted">
        <span className="h-2 w-2 animate-pulse rounded-full bg-success" aria-hidden="true" />
        <span aria-live="polite">Live</span>
      </span>
    );
  }
  if (state === "connecting" || state === "reconnecting") {
    return (
      <span className="inline-flex items-center gap-2 text-caption text-muted">
        <span className="h-2 w-2 animate-pulse rounded-full bg-warning" aria-hidden="true" />
        <span aria-live="polite">{state === "connecting" ? "Connecting…" : "Reconnecting…"}</span>
      </span>
    );
  }
  return null;
}
