// Runtime environment for the web app.
//
// Server-only vars must NOT be referenced from "use client" code; the
// `serverEnv` getter below throws if you try.
//
// Browser-safe vars must be prefixed with NEXT_PUBLIC_ at build time so
// Next.js inlines them. Their parsing happens lazily on first access.

const required = (name: string, value: string | undefined): string => {
  if (!value || value.length === 0) {
    throw new Error(`Missing required environment variable: ${name}`);
  }
  return value;
};

const optional = (value: string | undefined, fallback: string): string =>
  value && value.length > 0 ? value : fallback;

/** Server-only environment. Calling this from a client bundle throws. */
export function serverEnv() {
  if (typeof window !== "undefined") {
    throw new Error("serverEnv() called from the browser. Use publicEnv() instead.");
  }
  return {
    nodeEnv: optional(process.env.NODE_ENV, "development") as "development" | "test" | "production",
    /** Internal API base URL used by Server Components. */
    internalApiUrl: optional(process.env.INTERNAL_API_URL, "http://localhost:8080"),
    clerk: {
      secretKey: process.env.CLERK_SECRET_KEY,
      publishableKey: process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY,
    },
  };
}

/** Browser-safe environment. */
export function publicEnv() {
  return {
    /** Public API base URL used by client-side fetches. */
    apiUrl: optional(process.env.NEXT_PUBLIC_API_URL, "http://localhost:8080"),
    clerkPublishableKey: required(
      "NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY",
      process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY,
    ),
  };
}

/**
 * Returns true if Clerk is configured. Used to decide whether to render
 * sign-in/sign-out UI vs a "Configure auth" hint in development.
 */
export function isClerkConfigured(): boolean {
  return Boolean(process.env.NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY);
}
