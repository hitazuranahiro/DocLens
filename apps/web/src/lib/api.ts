// Server-side DocLens API client factory.
//
// Server Components and Route Handlers call `apiFromServer()` which
// builds a typed @doclens/api-client bound to INTERNAL_API_URL and
// authorized with the current request's Clerk session token. We never
// expose the secret key or session JWT to the browser bundle.

import { auth } from "@clerk/nextjs/server";
import { createDocLensClient } from "@doclens/api-client";
import { serverEnv } from "./env";

/**
 * Build a server-side API client for the current request. Calls live
 * inside Server Components, Route Handlers, or Server Actions.
 */
export async function apiFromServer() {
  const { internalApiUrl } = serverEnv();
  return createDocLensClient({
    baseUrl: internalApiUrl,
    tokenProvider: async () => {
      const { getToken } = await auth();
      const token = await getToken();
      // Empty token is acceptable on public endpoints; the API will
      // 401 protected ones and our callers handle that case.
      return token ?? "";
    },
  });
}
