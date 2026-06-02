// Next.js instrumentation hook.
//
// `register` runs once per server-process startup. We branch on
// runtime so the right Sentry config loads for the Node server vs
// the edge runtime.

export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    await import("./sentry.server.config");
  } else if (process.env.NEXT_RUNTIME === "edge") {
    await import("./sentry.edge.config");
  }
}

export { onRequestError } from "@sentry/nextjs";
