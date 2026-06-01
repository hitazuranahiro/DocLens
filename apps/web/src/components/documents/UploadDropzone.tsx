// UploadDropzone — drag/drop or click-to-pick a PDF, run the
// two-phase upload flow, and reflect per-file state.
//
// Validation runs both client-side (so the user finds out before any
// network round-trip) and server-side (the API is the source of truth
// — see Property 6 / Req 2.3). Client-side checks that match server
// rules are duplicated here on purpose: faster feedback, fewer wasted
// presigns.
"use client";

import { useCallback, useMemo, useState } from "react";
import { type FileRejection, useDropzone } from "react-dropzone";

import { useApiClient } from "@/lib/api-client";
import {
  type UploadInputFile,
  type UploadOutcome,
  UploadFailedError,
  uploadFile,
} from "@/lib/upload";

/** 100 MiB, mirroring the API's `MaxUploadBytes` constant. */
const MAX_BYTES = 100 * 1024 * 1024;

/** Default MIME allow-list. v0.1 ships PDF only. */
const DEFAULT_ACCEPTED_MIMES = ["application/pdf"] as const;

type ItemStatus =
  | { kind: "queued" }
  | { kind: "hashing" }
  | { kind: "uploading" }
  | { kind: "finalizing" }
  | { kind: "done"; outcome: UploadOutcome }
  | { kind: "error"; message: string };

interface QueueItem {
  id: string;
  file: File;
  status: ItemStatus;
}

export interface UploadDropzoneProps {
  /** Override the accepted MIME types. Defaults to PDF only. */
  acceptedMimes?: readonly string[];
  /** Override the size cap. Defaults to 100 MiB. */
  maxBytes?: number;
  /** Called once per successfully completed item. */
  onUploaded?: (outcome: UploadOutcome, file: File) => void;
}

/**
 * Drag-and-drop / click-to-pick uploader for the M3 pipeline.
 *
 * Manages an in-memory queue rather than a global store; on
 * navigation the queue resets, which is the right v0.1 behavior
 * (no resumable uploads yet).
 */
export function UploadDropzone({
  acceptedMimes = DEFAULT_ACCEPTED_MIMES,
  maxBytes = MAX_BYTES,
  onUploaded,
}: UploadDropzoneProps) {
  const client = useApiClient();
  const [queue, setQueue] = useState<QueueItem[]>([]);

  const accept = useMemo(() => buildAccept(acceptedMimes), [acceptedMimes]);

  const updateItem = useCallback((id: string, patch: Partial<Pick<QueueItem, "status">>) => {
    setQueue((prev) => prev.map((it) => (it.id === id ? { ...it, ...patch } : it)));
  }, []);

  const startUpload = useCallback(
    async (item: QueueItem) => {
      const { id, file } = item;
      try {
        updateItem(id, { status: { kind: "hashing" } });
        const bytes = await file.arrayBuffer();

        updateItem(id, { status: { kind: "uploading" } });
        const input: UploadInputFile = {
          filename: file.name,
          mimeType: file.type || "application/pdf",
          size: file.size,
          bytes,
        };

        const outcome = await uploadFile({ client, file: input });

        updateItem(id, { status: { kind: "done", outcome } });
        onUploaded?.(outcome, file);
      } catch (err) {
        const message = err instanceof UploadFailedError ? err.message : "upload failed";
        updateItem(id, { status: { kind: "error", message } });
      }
    },
    [client, onUploaded, updateItem],
  );

  const onDrop = useCallback(
    (accepted: File[], rejections: FileRejection[]) => {
      const nextItems: QueueItem[] = [
        ...rejections.map((r) => ({
          id: cryptoRandomId(),
          file: r.file,
          status: {
            kind: "error",
            message: humanizeRejection(r, maxBytes),
          } as ItemStatus,
        })),
        ...accepted.map((f) => ({
          id: cryptoRandomId(),
          file: f,
          status: { kind: "queued" } as ItemStatus,
        })),
      ];
      setQueue((prev) => [...prev, ...nextItems]);
      for (const item of nextItems) {
        if (item.status.kind === "queued") {
          // Run uploads in parallel; the API enforces dedupe so two
          // tabs racing the same file is also safe.
          void startUpload(item);
        }
      }
    },
    [maxBytes, startUpload],
  );

  const { getRootProps, getInputProps, isDragActive, isDragReject } = useDropzone({
    accept,
    maxSize: maxBytes,
    onDrop,
    multiple: true,
    noKeyboard: false,
  });

  return (
    <div className="space-y-6">
      <div
        {...getRootProps({
          className: dropzoneClass(isDragActive, isDragReject),
        })}
        aria-label="Upload documents"
      >
        <input {...getInputProps()} />
        <div className="flex flex-col items-center gap-2 text-center">
          <p className="text-base font-medium">
            {isDragActive
              ? isDragReject
                ? "That file type isn't supported"
                : "Drop to upload"
              : "Drag a PDF here, or click to choose"}
          </p>
          <p className="text-xs text-zinc-500 dark:text-zinc-400">
            {acceptedMimes.join(", ")} · up to {formatBytes(maxBytes)}
          </p>
        </div>
      </div>

      {queue.length > 0 && (
        <ul className="divide-y divide-zinc-200 rounded-lg border border-zinc-200 dark:divide-zinc-800 dark:border-zinc-800">
          {queue.map((item) => (
            <li key={item.id} className="flex items-center justify-between gap-4 px-4 py-3">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{item.file.name}</p>
                <p className="text-xs text-zinc-500 dark:text-zinc-400">
                  {formatBytes(item.file.size)}
                </p>
              </div>
              <StatusBadge status={item.status} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: ItemStatus }) {
  switch (status.kind) {
    case "queued":
      return <span className="text-xs text-zinc-500">Waiting…</span>;
    case "hashing":
      return <span className="text-xs text-zinc-500">Hashing…</span>;
    case "uploading":
      return <span className="text-xs text-zinc-500">Uploading…</span>;
    case "finalizing":
      return <span className="text-xs text-zinc-500">Finalizing…</span>;
    case "done":
      return (
        <span className="text-xs font-medium text-emerald-600 dark:text-emerald-400">
          {status.outcome.kind === "duplicate"
            ? "Already in your library"
            : "Queued for extraction"}
        </span>
      );
    case "error":
      return (
        <span className="text-xs font-medium text-red-600 dark:text-red-400">{status.message}</span>
      );
  }
}

function dropzoneClass(active: boolean, reject: boolean): string {
  const base =
    "flex min-h-[180px] cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed px-6 py-12 transition-colors";
  if (reject) {
    return `${base} border-red-400 bg-red-50/50 dark:bg-red-950/20`;
  }
  if (active) {
    return `${base} border-zinc-900 bg-zinc-100/60 dark:border-zinc-100 dark:bg-zinc-900/60`;
  }
  return `${base} border-zinc-300 hover:border-zinc-400 dark:border-zinc-700 dark:hover:border-zinc-500`;
}

function buildAccept(mimes: readonly string[]): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const m of mimes) {
    out[m] = [];
  }
  return out;
}

function humanizeRejection(rej: FileRejection, maxBytes: number): string {
  for (const e of rej.errors) {
    switch (e.code) {
      case "file-too-large":
        return `Larger than ${formatBytes(maxBytes)}`;
      case "file-invalid-type":
        return "File type not supported";
      case "file-too-small":
        return "File is empty";
    }
  }
  return rej.errors[0]?.message ?? "Rejected";
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`;
  return `${(n / (1024 * 1024)).toFixed(n < 10 * 1024 * 1024 ? 1 : 0)} MB`;
}

function cryptoRandomId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2);
}
