// Sentry server-side init.
//
// Loaded by Next.js automatically when @sentry/nextjs is installed
// and a sentry config file exists at the app root.
//
// We gate on NEXT_PUBLIC_SENTRY_DSN: an empty string disables
// Sentry entirely so local development and CI never ship telemetry.

import * as Sentry from "@sentry/nextjs";

const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN ?? "";

if (dsn) {
  Sentry.init({
    dsn,
    environment: process.env.NODE_ENV,
    release: process.env.NEXT_PUBLIC_APP_RELEASE,
    // Conservative tracing rate; raise after we have volume signal.
    tracesSampleRate: 0.1,
    // We don't want to ship request bodies or headers by default.
    sendDefaultPii: false,
  });
}
