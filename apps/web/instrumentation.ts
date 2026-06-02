// Next.js instrumentation hook.
//
// `register` runs once per server-process startup. We branch on
// runtime so the right Sentry config loads for the Node server vs
// the edge runtime, and (when configured) start the OTel tracer for
// the Node runtime.

import { registerOTel } from "@vercel/otel";

export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    await import("./sentry.server.config");

    // OpenTelemetry is env-gated: an empty endpoint means @vercel/otel
    // never registers, so the Node runtime stays cold-path-fast in
    // local dev and CI. When OTEL_EXPORTER_OTLP_ENDPOINT is set,
    // @vercel/otel ships traces to that collector via OTLP/HTTP.
    if (process.env.OTEL_EXPORTER_OTLP_ENDPOINT) {
      registerOTel({
        serviceName: "doclens-web",
        attributes: {
          "deployment.environment": process.env.NODE_ENV ?? "development",
          "service.version": process.env.NEXT_PUBLIC_APP_RELEASE ?? "",
        },
      });
    }
  } else if (process.env.NEXT_RUNTIME === "edge") {
    await import("./sentry.edge.config");
  }
}

export { onRequestError } from "@sentry/nextjs";
