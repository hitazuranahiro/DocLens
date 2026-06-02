// Sentry client-side init.
//
// Loads in the browser bundle when NEXT_PUBLIC_SENTRY_DSN is set;
// otherwise disabled so local builds carry no telemetry.

import * as Sentry from "@sentry/nextjs";

const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN ?? "";

if (dsn) {
  Sentry.init({
    dsn,
    environment: process.env.NODE_ENV,
    release: process.env.NEXT_PUBLIC_APP_RELEASE,
    tracesSampleRate: 0.1,
    sendDefaultPii: false,
    // Capture replays for 10% of sessions and 100% of error sessions.
    replaysSessionSampleRate: 0.1,
    replaysOnErrorSampleRate: 1.0,
  });
}
