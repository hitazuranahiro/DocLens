// Browser-facing Server-Sent Events proxy for live document_status
// updates.
//
// Why a Next.js Route Handler instead of opening EventSource directly
// against the API: EventSource cannot send custom headers (notably
// `Authorization: Bearer ...`), so the browser hits this same-origin
// route which authenticates via the Clerk session cookie, then opens
// an authenticated upstream connection and pipes the byte stream
// straight back.

import { auth } from "@clerk/nextjs/server";
import { serverEnv } from "@/lib/env";

// Force the Node.js runtime: we need streaming `fetch` over upstream
// HTTP/1.1 + ReadableStream forwarding. The Edge runtime supports it
// too but the Clerk SDK we use here is happier on Node.
export const runtime = "nodejs";
// Disable static optimization — this is a pure stream.
export const dynamic = "force-dynamic";

export async function GET(req: Request) {
  const { getToken } = await auth();
  const token = await getToken();
  if (!token) {
    return new Response("unauthenticated", { status: 401 });
  }
  const { internalApiUrl } = serverEnv();

  // The upstream connection lives as long as the client stays
  // attached. AbortController bridges the client-side disconnect to
  // an upstream cancel so we don't leak a goroutine on the API.
  const upstreamAborter = new AbortController();
  const cancelUpstream = () => upstreamAborter.abort();
  req.signal.addEventListener("abort", cancelUpstream, { once: true });

  let upstream: Response;
  try {
    upstream = await fetch(`${internalApiUrl}/v1/documents/stream`, {
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "text/event-stream",
      },
      signal: upstreamAborter.signal,
      cache: "no-store",
    });
  } catch (err) {
    req.signal.removeEventListener("abort", cancelUpstream);
    if (req.signal.aborted) {
      return new Response(null, { status: 499 });
    }
    return new Response(err instanceof Error ? err.message : "upstream unreachable", {
      status: 502,
    });
  }

  if (!upstream.ok || !upstream.body) {
    return new Response(null, { status: upstream.status || 502 });
  }

  return new Response(upstream.body, {
    status: 200,
    headers: {
      "Content-Type": "text/event-stream; charset=utf-8",
      "Cache-Control": "no-store",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no",
    },
  });
}
