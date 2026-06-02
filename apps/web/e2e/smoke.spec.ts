// End-to-end smoke test for v0.1.
//
// Exercises the full backend pipeline against a running compose
// stack: upload → finalize → wait for extraction → list → read →
// search → delete. Browser-driven Clerk sign-in is out of scope for
// CI in v0.1, so this test calls the API directly with the local
// auth provider's "dev:<userId>:<email>" tokens — which is exactly
// what the web client does after Clerk hands it a session JWT, so
// the contract under test is the same.
//
// Required env (CI sets these via docker-compose):
//   PLAYWRIGHT_API_URL  e.g. http://localhost:8080
//   PLAYWRIGHT_S3_URL   e.g. http://localhost:9000
//   AUTH_PROVIDER       must be 'local' on the API for this test

import { test, expect } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const API = process.env.PLAYWRIGHT_API_URL ?? "http://localhost:8080";
const TOKEN = `dev:e2e-user-${Date.now()}:e2e@doclens.test`;
const AUTH = { Authorization: `Bearer ${TOKEN}` };

// Fixture: a tiny one-page PDF the worker can process. Pre-committed
// under e2e/fixtures so the test is self-contained.
const FIXTURE_PATH = join(__dirname, "fixtures", "hello.pdf");

test.describe("v0.1 smoke", () => {
  test("upload → extract → search → delete", async ({ request }) => {
    const pdfBytes = readFileSync(FIXTURE_PATH);
    const sha256 = createHash("sha256").update(pdfBytes).digest("hex");
    const filename = `smoke-${Date.now()}.pdf`;

    // 1. Health check.
    const health = await request.get(`${API}/v1/health`);
    expect(health.ok()).toBeTruthy();

    // 2. /v1/me returns the local-auth identity.
    const me = await request.get(`${API}/v1/me`, { headers: AUTH });
    expect(me.ok()).toBeTruthy();
    const meBody = await me.json();
    expect(meBody.userId).toMatch(/^e2e-user-/);

    // 3. Request an upload slot.
    const create = await request.post(`${API}/v1/uploads`, {
      headers: { ...AUTH, "Content-Type": "application/json" },
      data: {
        filename,
        mimeType: "application/pdf",
        size: pdfBytes.length,
        sha256,
        title: "Smoke fixture",
      },
    });
    expect(create.status()).toBe(201);
    const slot = await create.json();
    expect(slot.uploadId).toBeTruthy();
    expect(slot.putUrl).toMatch(/^http/);

    // 4. PUT the bytes to MinIO via the presigned URL.
    const put = await request.fetch(slot.putUrl, {
      method: "PUT",
      headers: { "Content-Type": "application/pdf" },
      data: pdfBytes,
    });
    expect(put.ok()).toBeTruthy();

    // 5. Finalize the upload — server checks the object landed.
    const finalize = await request.post(`${API}/v1/documents/${slot.uploadId}/finalize`, {
      headers: AUTH,
    });
    expect(finalize.status()).toBe(202);
    const doc = await finalize.json();
    expect(doc.id).toBeTruthy();
    expect(["queued", "extracting"]).toContain(doc.status);

    // 6. Poll until the worker flips status to ready (or fail).
    const documentId = doc.id as string;
    const ready = await pollFor(async () => {
      const r = await request.get(`${API}/v1/documents/${documentId}`, { headers: AUTH });
      if (!r.ok()) return null;
      const body = await r.json();
      if (body.document.status === "ready") return body.document;
      if (body.document.status === "failed") {
        throw new Error(`extraction failed: ${body.document.lastError}`);
      }
      return null;
    }, 60_000);

    expect(ready.status).toBe("ready");
    expect(ready.pageCount ?? 0).toBeGreaterThan(0);
    expect(ready.wordCount ?? 0).toBeGreaterThan(0);

    // 7. Markdown is fetchable.
    const md = await request.get(`${API}/v1/documents/${documentId}/markdown`, {
      headers: AUTH,
    });
    expect(md.ok()).toBeTruthy();
    const mdBody = await md.text();
    expect(mdBody.length).toBeGreaterThan(0);

    // 8. The document shows up in the library list.
    const list = await request.get(`${API}/v1/documents`, { headers: AUTH });
    expect(list.ok()).toBeTruthy();
    const listBody = await list.json();
    expect(listBody.items.find((d: { id: string }) => d.id === documentId)).toBeTruthy();

    // 9. Search hits the document. We pull a token from the markdown
    //    so this test stays independent of the fixture's exact wording.
    const probe = pickSearchProbe(mdBody);
    if (probe) {
      const search = await request.get(`${API}/v1/search?q=${encodeURIComponent(probe)}`, {
        headers: AUTH,
      });
      expect(search.ok()).toBeTruthy();
      const searchBody = await search.json();
      expect(
        searchBody.items.find((h: { documentId: string }) => h.documentId === documentId),
      ).toBeTruthy();
    }

    // 10. Delete returns 204.
    const del = await request.delete(`${API}/v1/documents/${documentId}`, { headers: AUTH });
    expect(del.status()).toBe(204);

    // 11. The document is gone from the list.
    const listAfter = await request.get(`${API}/v1/documents`, { headers: AUTH });
    const listAfterBody = await listAfter.json();
    expect(listAfterBody.items.find((d: { id: string }) => d.id === documentId)).toBeFalsy();

    // 12. And gone from search.
    if (probe) {
      const searchAfter = await request.get(`${API}/v1/search?q=${encodeURIComponent(probe)}`, {
        headers: AUTH,
      });
      const searchAfterBody = await searchAfter.json();
      expect(
        searchAfterBody.items.find((h: { documentId: string }) => h.documentId === documentId),
      ).toBeFalsy();
    }
  });
});

async function pollFor<T>(fn: () => Promise<T | null>, timeoutMs: number): Promise<T> {
  const deadline = Date.now() + timeoutMs;
  let lastErr: unknown = null;
  while (Date.now() < deadline) {
    try {
      const r = await fn();
      if (r !== null) return r;
    } catch (err) {
      lastErr = err;
    }
    await new Promise((res) => setTimeout(res, 1_000));
  }
  throw new Error(`pollFor timed out after ${timeoutMs}ms${lastErr ? `: ${String(lastErr)}` : ""}`);
}

// pickSearchProbe extracts the first meaningful word from the
// extracted markdown so the search assertion doesn't rely on any
// specific fixture content. Skips numbers, single chars, and
// markdown punctuation.
function pickSearchProbe(md: string): string | null {
  for (const raw of md.split(/\s+/)) {
    const w = raw.replace(/[^a-z]/gi, "").toLowerCase();
    if (w.length >= 4 && /^[a-z]+$/.test(w)) {
      return w;
    }
  }
  return null;
}
