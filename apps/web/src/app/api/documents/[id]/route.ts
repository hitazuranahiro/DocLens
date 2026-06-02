// Browser-facing DELETE proxy. Mirrors the markdown/thumbnail
// pattern: the API needs a Bearer token, the browser only carries
// the Clerk session cookie, so this same-origin route bridges the
// two and forwards the upstream status.

import { NextResponse } from "next/server";

import { auth } from "@clerk/nextjs/server";
import { serverEnv } from "@/lib/env";

export const runtime = "nodejs";

export async function DELETE(_req: Request, { params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!isUUID(id)) {
    return NextResponse.json({ error: "bad id" }, { status: 400 });
  }
  const { getToken } = await auth();
  const token = await getToken();
  if (!token) {
    return NextResponse.json({ error: "unauthenticated" }, { status: 401 });
  }
  const { internalApiUrl } = serverEnv();
  const upstream = await fetch(`${internalApiUrl}/v1/documents/${id}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!upstream.ok && upstream.status !== 204) {
    return new NextResponse(null, { status: upstream.status });
  }
  return new NextResponse(null, { status: 204 });
}

function isUUID(s: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(s);
}
