// Root layout. Wraps every route in a Clerk provider so server-side
// `auth()` and the client-side `<UserButton/>` resolve consistently.
//
// Metadata defaults live here; route segments override per-page values.

import type { Metadata } from "next";
import { ClerkProvider } from "@clerk/nextjs";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "DocLens",
    template: "%s · DocLens",
  },
  description: "Turn documents into searchable, AI-ready knowledge.",
  metadataBase: new URL(process.env.NEXT_PUBLIC_SITE_URL ?? "http://localhost:3000"),
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <ClerkProvider>
      <html lang="en">
        <body className="min-h-screen antialiased">{children}</body>
      </html>
    </ClerkProvider>
  );
}
