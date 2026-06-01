// React hook around EventSource for live document_status updates.
//
// Contract:
//
//   const { events, status } = useDocumentStream();
//
// `events` is a monotonically growing list (newest last) of payloads
// the API has flushed since the connection opened. `status` lets the
// UI show a "live" indicator or a soft "reconnecting…" state.
//
// EventSource handles reconnection automatically; we just expose the
// readyState. The hook is safe to mount/unmount: on unmount we close
// the connection, which lets the Next proxy route abort upstream.

"use client";

import { useEffect, useRef, useState } from "react";

import type { components } from "@doclens/api-client";

export type DocumentStatusEvent = NonNullable<components["schemas"]["DocumentStatusEvent"]>;

export type StreamConnectionState = "connecting" | "open" | "reconnecting" | "closed";

interface UseDocumentStreamOptions {
  /**
   * URL to subscribe to. Defaults to the same-origin Next proxy at
   * `/api/documents/stream`.
   */
  url?: string;
  /**
   * Disables the hook entirely. Useful for tests or when the page
   * wants to opt out of live updates.
   */
  disabled?: boolean;
}

export interface UseDocumentStreamResult {
  /** Latest event seen; `null` until the first one arrives. */
  lastEvent: DocumentStatusEvent | null;
  /** Connection state for UI indicators. */
  state: StreamConnectionState;
}

export function useDocumentStream(
  onEvent: (ev: DocumentStatusEvent) => void,
  options: UseDocumentStreamOptions = {},
): UseDocumentStreamResult {
  const { url = "/api/documents/stream", disabled = false } = options;
  const [state, setState] = useState<StreamConnectionState>("closed");
  const [lastEvent, setLastEvent] = useState<DocumentStatusEvent | null>(null);
  // Hold the latest callback in a ref so the EventSource effect can
  // keep its identity while components re-render with new closures.
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  useEffect(() => {
    if (disabled) {
      setState("closed");
      return;
    }
    if (typeof window === "undefined" || typeof EventSource === "undefined") {
      return;
    }

    setState("connecting");
    const source = new EventSource(url);

    source.onopen = () => setState("open");
    source.onerror = () => {
      // EventSource transitions to CONNECTING (0) automatically while
      // it backs off. We surface that as "reconnecting" so the UI can
      // show a soft cue without the user thinking we're dead.
      if (source.readyState === EventSource.CLOSED) {
        setState("closed");
      } else {
        setState("reconnecting");
      }
    };
    source.onmessage = (msg) => {
      if (!msg.data) return;
      try {
        const parsed = JSON.parse(msg.data) as DocumentStatusEvent;
        if (!parsed.documentId || !parsed.status) return;
        setLastEvent(parsed);
        onEventRef.current(parsed);
      } catch {
        // Ignore malformed payloads. The server only writes typed JSON,
        // so malformed lines mean a misbehaving proxy.
      }
    };

    return () => {
      source.close();
      setState("closed");
    };
  }, [url, disabled]);

  return { lastEvent, state };
}
