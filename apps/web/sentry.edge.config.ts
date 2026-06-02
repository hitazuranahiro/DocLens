// Sentry edge runtime init (Next.js middleware + edge route handlers).
//
// We don't run anything heavy on the edge in v0.1, but Next.js
// requires this file to exist when @sentry/nextjs is installed.

import * as Sentry from "@sentry/nextjs";

const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN ?? "";

if (dsn) {
  Sentry.init({
    dsn,
    environment: process.env.NODE_ENV,
    release: process.env.NEXT_PUBLIC_APP_RELEASE,
    tracesSampleRate: 0.1,
  });
}
