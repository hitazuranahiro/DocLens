// Browser-facing markdown proxy. Same pattern as the thumbnail
// route: the API needs a Bearer token, the browser only has a
// Clerk session cookie, so this route bridges the two.

import { NextResponse } from "next/server";

import { auth } from "@clerk/nextjs/server";
import { serverEnv } from "@/lib/env";

export async function GET(_req: Request, { params }: { params: Promise<{ id: string }> }) {
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
  const upstream = await fetch(`${internalApiUrl}/v1/documents/${id}/markdown`, {
    headers: { Authorization: `Bearer ${token}` },
    cache: "no-store",
  });

  if (!upstream.ok) {
    return new NextResponse(null, { status: upstream.status });
  }
  return new NextResponse(upstream.body, {
    status: 200,
    headers: {
      "Content-Type": "text/markdown; charset=utf-8",
    },
  });
}

function isUUID(s: string): boolean {
  return /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(s);
}
