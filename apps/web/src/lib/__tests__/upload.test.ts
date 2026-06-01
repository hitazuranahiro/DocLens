// Tests for the pure upload flow.
//
// We don't go through @clerk/nextjs or @doclens/api-client here.
// The flow takes the client as a parameter so we substitute a small
// fake. The PUT fetcher is also injectable.

import { describe, expect, it, vi } from "vitest";

import { type ApiClient } from "../api-client";
import {
  type PutFetcher,
  type UploadInputFile,
  UploadFailedError,
  sha256Hex,
  uploadFile,
} from "../upload";

// --- helpers --------------------------------------------------------------

function pdfFile(): UploadInputFile {
  const bytes = new TextEncoder().encode("%PDF-1.7\nfake pdf bytes\n");
  return {
    filename: "doc.pdf",
    mimeType: "application/pdf",
    size: bytes.byteLength,
    bytes: bytes.buffer,
  };
}

function makeFakeClient(opts: {
  upload: { status: number; data?: unknown; error?: unknown };
  finalize?: { status: number; data?: unknown; error?: unknown };
}) {
  const post = vi.fn(async (path: string) => {
    if (path === "/v1/uploads") {
      return {
        data: opts.upload.data,
        error: opts.upload.error,
        response: new Response(null, { status: opts.upload.status }),
      };
    }
    if (path === "/v1/documents/{id}/finalize") {
      const f = opts.finalize ?? { status: 202, data: { id: "doc-1" } };
      return {
        data: f.data,
        error: f.error,
        response: new Response(null, { status: f.status }),
      };
    }
    throw new Error(`unexpected path ${path}`);
  });
  return { post, client: { POST: post } as unknown as ApiClient };
}

const okPut: PutFetcher = async () => new Response(null, { status: 200 });

// --- tests ----------------------------------------------------------------

describe("sha256Hex", () => {
  it("produces a stable lowercase 64-char hex digest", async () => {
    const empty = await sha256Hex(new ArrayBuffer(0));
    expect(empty).toBe("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855");
    expect(empty).toMatch(/^[a-f0-9]{64}$/);
  });
});

describe("uploadFile", () => {
  it("runs presign → PUT → finalize on a fresh upload", async () => {
    const presigned = "https://signed.example/raw/u/doc-1/doc.pdf";
    const { post, client } = makeFakeClient({
      upload: {
        status: 201,
        data: {
          documentId: "doc-1",
          uploadId: "upl-1",
          putUrl: presigned,
          expiresAt: new Date().toISOString(),
          status: "pending",
        },
      },
    });
    const put = vi.fn(okPut);

    const out = await uploadFile({ client, file: pdfFile(), putFetcher: put });

    expect(out).toEqual({ kind: "uploaded", documentId: "doc-1" });
    expect(post).toHaveBeenCalledTimes(2);
    expect(post.mock.calls[0]?.[0]).toBe("/v1/uploads");
    expect(post.mock.calls[1]?.[0]).toBe("/v1/documents/{id}/finalize");
    expect(put).toHaveBeenCalledOnce();
    const [putUrl, putInit] = put.mock.calls[0] ?? [];
    expect(putUrl).toBe(presigned);
    expect((putInit as RequestInit).method).toBe("PUT");
  });

  it("short-circuits on duplicates without PUT or finalize", async () => {
    const { post, client } = makeFakeClient({
      upload: {
        status: 200,
        data: { documentId: "doc-existing", status: "ready" },
      },
    });
    const put = vi.fn(okPut);

    const out = await uploadFile({ client, file: pdfFile(), putFetcher: put });

    expect(out).toEqual({ kind: "duplicate", documentId: "doc-existing" });
    expect(post).toHaveBeenCalledTimes(1);
    expect(put).not.toHaveBeenCalled();
  });

  it("surfaces 415 as an unsupported-mime error", async () => {
    const { client } = makeFakeClient({
      upload: {
        status: 415,
        error: { title: "Unsupported", detail: "this MIME is not enabled" },
      },
    });
    await expect(
      uploadFile({ client, file: pdfFile(), putFetcher: vi.fn(okPut) }),
    ).rejects.toMatchObject({
      name: "UploadFailedError",
      reason: "unsupported-mime",
      status: 415,
    });
  });

  it("surfaces 413 as a too-large error", async () => {
    const { client } = makeFakeClient({
      upload: { status: 413, error: { title: "Payload too large" } },
    });
    await expect(
      uploadFile({ client, file: pdfFile(), putFetcher: vi.fn(okPut) }),
    ).rejects.toMatchObject({ reason: "too-large", status: 413 });
  });

  it("surfaces 401 as an unauthorized error and never PUTs", async () => {
    const { client } = makeFakeClient({
      upload: { status: 401, error: { title: "Unauthorized" } },
    });
    const put = vi.fn(okPut);
    await expect(uploadFile({ client, file: pdfFile(), putFetcher: put })).rejects.toMatchObject({
      reason: "unauthorized",
      status: 401,
    });
    expect(put).not.toHaveBeenCalled();
  });

  it("surfaces a non-2xx PUT as a network error", async () => {
    const { client } = makeFakeClient({
      upload: {
        status: 201,
        data: {
          documentId: "doc-1",
          uploadId: "upl-1",
          putUrl: "https://signed.example/x",
          status: "pending",
        },
      },
    });
    const put = vi.fn(async () => new Response(null, { status: 403 }));
    await expect(uploadFile({ client, file: pdfFile(), putFetcher: put })).rejects.toMatchObject({
      reason: "network",
      status: 403,
    });
  });

  it("surfaces 409 from finalize as object-missing", async () => {
    const { client } = makeFakeClient({
      upload: {
        status: 201,
        data: {
          documentId: "doc-1",
          uploadId: "upl-1",
          putUrl: "https://signed.example/x",
          status: "pending",
        },
      },
      finalize: { status: 409, error: { title: "Object not present" } },
    });
    await expect(
      uploadFile({ client, file: pdfFile(), putFetcher: vi.fn(okPut) }),
    ).rejects.toMatchObject({ reason: "object-missing", status: 409 });
  });

  it("rejects 201 responses missing a putUrl", async () => {
    const { client } = makeFakeClient({
      upload: {
        status: 201,
        data: { documentId: "doc-1", uploadId: "upl-1", status: "pending" },
      },
    });
    await expect(
      uploadFile({ client, file: pdfFile(), putFetcher: vi.fn(okPut) }),
    ).rejects.toBeInstanceOf(UploadFailedError);
  });
});
