// Browser-facing thumbnail proxy. The API requires a Bearer token,
// which the browser cannot mint from a Clerk session cookie alone;
// this route runs in the user's session, attaches the token via
// `apiFromServer()`, and pipes the image bytes back.
//
// Cache headers come from the upstream response when present.

import { NextResponse } from "next/server";

import { apiFromServer } from "@/lib/api";
import { serverEnv } from "@/lib/env";
import { auth } from "@clerk/nextjs/server";

export async function GET(_req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!isUUID(id)) {
    return NextResponse.json({ error: "bad id" }, { status: 400 });
  }

  // We don't use the typed client here because openapi-fetch decodes
  // bodies for us; we need raw bytes. Mint a token via Clerk and
  // call the API directly.
  const { getToken } = await auth();
  const token = await getToken();
  if (!token) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const { internalApiUrl } = serverEnv();
  const upstream = await fetch(`${internalApiUrl}/v1/documents/${id}/thumbnail`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!upstream.ok) {
    return new NextResponse(null, { status: upstream.status });
  }

  return new NextResponse(upstream.body, {
    status: 200,
    headers: {
      "Content-Type": upstream.headers.get("Content-Type") ?? "image/png",
      "Cache-Control": upstream.headers.get("Cache-Control") ?? "private, max-age=300",
    },
  });
}

// Avoid bundling the @doclens/api-client typed client in the route's
// runtime; we only used it for `apiFromServer` typing, drop it.
void apiFromServer;

function isUUID(s: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(s);
}
