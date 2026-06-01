// Next.js middleware. Runs at the edge for every matched request and
// gates the protected `(app)` route group behind Clerk auth.
//
// Public routes pass through untouched so the marketing pages and the
// Clerk sign-in / sign-up flows can render without a session.

import { clerkMiddleware, createRouteMatcher } from "@clerk/nextjs/server";

const isProtected = createRouteMatcher(["/library(.*)", "/upload(.*)", "/search(.*)"]);

export default clerkMiddleware(async (auth, req) => {
  if (isProtected(req)) {
    await auth.protect();
  }
});

export const config = {
  matcher: [
    // Skip Next internals and static assets unless they have search params
    "/((?!_next|[^?]*\\.(?:html?|css|js(?!on)|jpe?g|webp|png|gif|svg|ttf|woff2?|ico|csv|docx?|xlsx?|zip|webmanifest)).*)",
    // Always run for API routes
    "/(api|trpc)(.*)",
  ],
};
