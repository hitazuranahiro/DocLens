// Marketing layout. Public, unauthenticated. Thin top nav with a
// sign-in CTA. Designed to keep dependencies and bundle size minimal.

import Link from "next/link";
import { SignedIn, SignedOut, UserButton } from "@clerk/nextjs";

export default function MarketingLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col bg-bg">
      <header className="border-b border-border">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-6">
          <Link href="/" className="text-title text-text-strong tracking-tight">
            DocLens
          </Link>
          <nav className="flex items-center gap-4 text-label">
            <Link
              href="https://github.com/hitazuranahiro/DocLens"
              className="text-muted transition-colors duration-base hover:text-text-strong"
            >
              GitHub
            </Link>
            <SignedOut>
              <Link
                href="/sign-in"
                className="rounded-sm bg-brand px-3 py-1.5 text-white transition-opacity duration-base hover:opacity-90"
              >
                Sign in
              </Link>
            </SignedOut>
            <SignedIn>
              <Link
                href="/library"
                className="text-muted transition-colors duration-base hover:text-text-strong"
              >
                Library
              </Link>
              <UserButton afterSignOutUrl="/" />
            </SignedIn>
          </nav>
        </div>
      </header>
      <main className="flex-1">{children}</main>
    </div>
  );
}
