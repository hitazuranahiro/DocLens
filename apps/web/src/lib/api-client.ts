// Browser-side DocLens API client.
//
// `useApiClient()` returns a typed @doclens/api-client bound to the
// public API URL and authorized via Clerk's client-side getToken().
// It is memoized per session so refetching cheap. The token is
// resolved on every request through the middleware, so refreshes
// happen transparently.
"use client";

import { useAuth } from "@clerk/nextjs";
import { createDocLensClient } from "@doclens/api-client";
import { useMemo } from "react";

import { publicEnv } from "./env";

export type ApiClient = ReturnType<typeof createDocLensClient>;

/** Hook returning a memoized client for the current Clerk session. */
export function useApiClient(): ApiClient {
  const { getToken } = useAuth();
  return useMemo(
    () =>
      createDocLensClient({
        baseUrl: publicEnv().apiUrl,
        tokenProvider: async () => (await getToken()) ?? "",
      }),
    [getToken],
  );
}
