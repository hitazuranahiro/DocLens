// Two-phase document upload, browser-side.
//
//   1. POST /v1/uploads — server validates and either returns 200 with
//      an existing documentId (duplicate) or 201 with a presigned PUT URL.
//   2. PUT bytes directly to object storage. The browser sees the URL
//      but never the API server's bytes.
//   3. POST /v1/documents/{uploadId}/finalize — server HEADs the object
//      and inserts the canonical Library row.
//
// This module is pure logic. The component (`UploadDropzone`) supplies
// the API client and an optional `putFetcher` (so tests can substitute
// fetch). Errors are typed so the component renders a useful message.

import type { ApiClient } from "./api-client";

/** Browser-friendly file metadata. */
export interface UploadInputFile {
  filename: string;
  mimeType: string;
  size: number;
  bytes: ArrayBuffer;
  /** Optional human-readable title; defaults to filename without extension. */
  title?: string;
}

export type UploadOutcome =
  | { kind: "uploaded"; documentId: string }
  | { kind: "duplicate"; documentId: string };

/** Failure modes the dropzone surfaces to the user. */
export type UploadErrorReason =
  | "unsupported-mime"
  | "too-large"
  | "bad-request"
  | "unauthorized"
  | "object-missing"
  | "network"
  | "server"
  | "missing-put-url";

export class UploadFailedError extends Error {
  public readonly reason: UploadErrorReason;
  public readonly status: number | null;
  public constructor(reason: UploadErrorReason, message: string, status: number | null = null) {
    super(message);
    this.name = "UploadFailedError";
    this.reason = reason;
    this.status = status;
  }
}

/** Optional progress callback. `loaded` and `total` are byte counts. */
export type ProgressFn = (loaded: number, total: number) => void;

/** Optional fetcher used for the PUT step. Defaults to the global fetch. */
export type PutFetcher = (
  url: string,
  init: RequestInit,
  onProgress?: ProgressFn,
) => Promise<Response>;

export interface UploadOptions {
  client: ApiClient;
  file: UploadInputFile;
  signal?: AbortSignal;
  onProgress?: ProgressFn;
  /** Used by tests to substitute a fake PUT. */
  putFetcher?: PutFetcher;
}

/**
 * Compute lowercase hex SHA-256 of bytes using the Web Crypto API.
 *
 * Throws if `crypto.subtle` is unavailable (i.e. non-HTTPS context).
 */
export async function sha256Hex(bytes: ArrayBuffer): Promise<string> {
  if (typeof crypto === "undefined" || !crypto.subtle) {
    throw new Error("Web Crypto unavailable; HTTPS or localhost is required");
  }
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

/**
 * Run the full upload flow for a single file. Returns whether the
 * server treated the upload as a fresh upload or as a duplicate of an
 * existing document.
 */
export async function uploadFile(opts: UploadOptions): Promise<UploadOutcome> {
  const { client, file, signal, onProgress, putFetcher = defaultPutFetcher } = opts;

  const sha256 = await sha256Hex(file.bytes);

  // Step 1: POST /v1/uploads.
  const created = await client.POST("/v1/uploads", {
    body: {
      filename: file.filename,
      mimeType: file.mimeType,
      size: file.size,
      sha256,
      title: file.title,
    },
    signal,
  });

  if (created.error || !created.data) {
    throw mapApiError(created.response.status, created.error);
  }

  // 200 = duplicate. The server already has this file for this user;
  // we never PUT and never call /finalize.
  if (created.response.status === 200) {
    return { kind: "duplicate", documentId: created.data.documentId };
  }

  // 201 = fresh upload. We must have a putUrl and an uploadId.
  const { putUrl, uploadId, documentId } = created.data;
  if (!putUrl || !uploadId) {
    throw new UploadFailedError("missing-put-url", "API returned 201 without a presigned URL");
  }

  // Step 2: PUT the bytes.
  const putResp = await putFetcher(
    putUrl,
    {
      method: "PUT",
      headers: { "Content-Type": file.mimeType },
      body: file.bytes,
      signal,
    },
    onProgress,
  );
  if (!putResp.ok) {
    throw new UploadFailedError(
      "network",
      `Object storage rejected the PUT (status ${putResp.status})`,
      putResp.status,
    );
  }

  // Step 3: POST /v1/documents/{uploadId}/finalize.
  const finalized = await client.POST("/v1/documents/{id}/finalize", {
    params: { path: { id: uploadId } },
    signal,
  });
  if (finalized.error || !finalized.data) {
    throw mapApiError(finalized.response.status, finalized.error);
  }

  return { kind: "uploaded", documentId };
}

/**
 * Default PUT fetcher. Streams the body without progress events; the
 * Fetch API doesn't expose upload progress in modern browsers without
 * Streams + WritableStream tee, which most files don't justify. The
 * dropzone shows determinate progress for hash + indeterminate during
 * the PUT, which matches user expectations for sub-100 MB files.
 */
async function defaultPutFetcher(url: string, init: RequestInit): Promise<Response> {
  return fetch(url, init);
}

function mapApiError(status: number, problem: unknown): UploadFailedError {
  const detail = readDetail(problem) ?? "request failed";
  switch (status) {
    case 401:
      return new UploadFailedError("unauthorized", detail, status);
    case 413:
      return new UploadFailedError("too-large", detail, status);
    case 415:
      return new UploadFailedError("unsupported-mime", detail, status);
    case 409:
      return new UploadFailedError("object-missing", detail, status);
    case 400:
      return new UploadFailedError("bad-request", detail, status);
    default:
      if (status >= 500) {
        return new UploadFailedError("server", detail, status);
      }
      return new UploadFailedError("server", detail, status);
  }
}

function readDetail(problem: unknown): string | null {
  if (problem && typeof problem === "object") {
    const p = problem as { detail?: unknown; title?: unknown };
    if (typeof p.detail === "string" && p.detail.length > 0) return p.detail;
    if (typeof p.title === "string" && p.title.length > 0) return p.title;
  }
  return null;
}
