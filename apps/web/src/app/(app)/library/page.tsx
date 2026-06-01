// Library landing page. Verifies the auth bridge end-to-end by calling
// `/v1/me` with the current Clerk session token from a Server Component
// and rendering the resolved identity. Subsequent milestones replace
// the body of this page with the real document list.
//
// This is also where the requirement "Server Component that calls
// /v1/me server-side and renders the user" lands (Task 2.4).

import { apiFromServer } from "@/lib/api";

export const dynamic = "force-dynamic";

export default async function LibraryPage() {
  const client = await apiFromServer();
  const { data, error, response } = await client.GET("/v1/me");

  if (error || !data) {
    return (
      <div className="space-y-4">
        <h1 className="text-2xl font-semibold tracking-tight">Library</h1>
        <p className="text-sm text-red-600 dark:text-red-400">
          Failed to load identity ({response?.status ?? "no response"}).
        </p>
        <pre className="rounded-md bg-zinc-100 p-4 text-xs text-zinc-700 dark:bg-zinc-900 dark:text-zinc-300">
          {error ? JSON.stringify(error, null, 2) : "Unknown error"}
        </pre>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Library</h1>
        <p className="text-sm text-zinc-600 dark:text-zinc-300">
          Signed in as <span className="font-medium">{data.displayName ?? data.email}</span>{" "}
          <span className="text-zinc-400">({data.userId})</span>
        </p>
      </header>

      <div className="rounded-lg border border-dashed border-zinc-300 p-12 text-center dark:border-zinc-700">
        <p className="text-sm text-zinc-600 dark:text-zinc-300">
          No documents yet. Upload pipeline ships in M3.
        </p>
      </div>
    </div>
  );
}
