// Root layout. Wraps every route in a Clerk provider so server-side
// `auth()` and the client-side `<UserButton/>` resolve consistently.
//
// Next/font loads Inter and JetBrains Mono at build time, exposes them
// as CSS variables, and the global stylesheet maps them to --font-sans
// and --font-mono. That keeps the design token contract intact.

import type { Metadata } from "next";
import { ClerkProvider } from "@clerk/nextjs";
import { dark } from "@clerk/themes";
import { Inter, JetBrains_Mono } from "next/font/google";

import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

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
    <ClerkProvider
      appearance={{
        baseTheme: dark,
        variables: {
          // Clerk reads its tokens from CSS-resolvable values. We pin
          // the brand purple so the auth flows match DocLens.
          colorPrimary: "#6C47FF",
          colorBackground: "#131316",
          colorInputBackground: "#1E1E26",
          colorInputText: "#FFFFFF",
          colorText: "#FFFFFF",
          colorTextSecondary: "#7C7C99",
          borderRadius: "10px",
          fontFamily: "var(--font-inter), ui-sans-serif, system-ui",
        },
        elements: {
          card: "shadow-md",
          formButtonPrimary: "bg-[#6C47FF] hover:bg-[#5b3aef] text-white font-medium",
        },
      }}
    >
      <html lang="en" className={`${inter.variable} ${jetbrainsMono.variable}`}>
        <body className="min-h-screen antialiased">{children}</body>
      </html>
    </ClerkProvider>
  );
}
