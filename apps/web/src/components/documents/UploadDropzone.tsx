// UploadDropzone — drag/drop or click to pick a PDF, run the
// two-phase upload flow, and reflect per-file state.
//
// Validation runs both client-side (so the user finds out before any
// network round-trip) and server-side (the API is the source of
// truth — see Property 6 / Req 2.3). Visual tokens follow DESIGN.md:
// dashed border at radius-lg, brand-purple on drag-over, error-red
// on drag-reject.
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
  acceptedMimes?: readonly string[];
  maxBytes?: number;
  onUploaded?: (outcome: UploadOutcome, file: File) => void;
}

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
          <p className="text-title text-text-strong">
            {isDragActive
              ? isDragReject
                ? "That file type isn't supported"
                : "Drop to upload"
              : "Drag a PDF here, or click to choose"}
          </p>
          <p className="text-caption text-muted">
            {acceptedMimes.join(", ")} · up to {formatBytes(maxBytes)}
          </p>
        </div>
      </div>

      {queue.length > 0 && (
        <ul className="divide-y divide-border rounded-md border border-border bg-surface">
          {queue.map((item) => (
            <li key={item.id} className="flex items-center justify-between gap-4 px-4 py-3">
              <div className="min-w-0 flex-1">
                <p className="truncate text-label text-text-strong">{item.file.name}</p>
                <p className="text-caption text-muted">{formatBytes(item.file.size)}</p>
              </div>
              <StatusPill status={item.status} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function StatusPill({ status }: { status: ItemStatus }) {
  switch (status.kind) {
    case "queued":
      return <span className="text-caption text-muted">Waiting…</span>;
    case "hashing":
      return <span className="text-caption text-muted">Hashing…</span>;
    case "uploading":
      return <span className="text-caption text-info">Uploading…</span>;
    case "finalizing":
      return <span className="text-caption text-info">Finalizing…</span>;
    case "done":
      return (
        <span className="text-caption font-medium text-success">
          {status.outcome.kind === "duplicate"
            ? "Already in your library"
            : "Queued for extraction"}
        </span>
      );
    case "error":
      return <span className="text-caption font-medium text-error">{status.message}</span>;
  }
}

function dropzoneClass(active: boolean, reject: boolean): string {
  const base =
    "flex min-h-[180px] cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed px-6 py-12 transition-colors duration-base";
  if (reject) {
    return `${base} border-error bg-error-surface`;
  }
  if (active) {
    return `${base} border-brand bg-brand/10`;
  }
  return `${base} border-border bg-surface hover:border-gray-400`;
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
