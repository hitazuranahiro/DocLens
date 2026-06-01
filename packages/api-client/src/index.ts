// Public surface of @doclens/api-client.
//
// The generated `paths` types describe every operation in openapi.yaml.
// `createDocLensClient` returns a typed fetch wrapper bound to a base URL.
//
// Usage:
//   const client = createDocLensClient({ baseUrl: "http://localhost:8080" });
//   const { data, error } = await client.GET("/v1/health");

import createClient, { type ClientOptions, type Middleware } from "openapi-fetch";

import type { paths, components } from "./generated/schema.js";

export type { paths, components };

/** Convenience aliases for the most-used schemas. */
export type Health = components["schemas"]["Health"];
export type Identity = components["schemas"]["Identity"];
export type Problem = components["schemas"]["Problem"];

export interface DocLensClientOptions extends ClientOptions {
  /** Base URL of the DocLens API. */
  baseUrl: string;
  /** Static bearer token. Use `tokenProvider` for refresh-aware flows. */
  token?: string;
  /** Async getter called on every request; supersedes `token` when set. */
  tokenProvider?: () => string | Promise<string>;
}

/** Create a typed DocLens API client. */
export function createDocLensClient(opts: DocLensClientOptions) {
  const client = createClient<paths>({ baseUrl: opts.baseUrl });
  const auth: Middleware = {
    async onRequest({ request }) {
      const token = opts.tokenProvider ? await opts.tokenProvider() : opts.token;
      if (token) {
        request.headers.set("Authorization", `Bearer ${token}`);
      }
      return request;
    },
  };
  client.use(auth);
  return client;
}
