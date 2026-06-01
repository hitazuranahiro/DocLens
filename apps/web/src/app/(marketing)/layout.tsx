// Marketing layout. Public, unauthenticated. Renders a thin top nav
// with sign-in CTA. Designed to keep dependencies and bundle size
// minimal so the landing page is fast.

import Link from "next/link";
import { SignedIn, SignedOut, UserButton } from "@clerk/nextjs";

export default function MarketingLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-b border-zinc-200 dark:border-zinc-800">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-6">
          <Link href="/" className="font-semibold tracking-tight">
            DocLens
          </Link>
          <nav className="flex items-center gap-4 text-sm">
            <Link
              href="https://github.com/hitazuranahiro/DocLens"
              className="text-zinc-600 hover:text-zinc-900 dark:text-zinc-300 dark:hover:text-zinc-100"
            >
              GitHub
            </Link>
            <SignedOut>
              <Link
                href="/sign-in"
                className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-zinc-800 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-200"
              >
                Sign in
              </Link>
            </SignedOut>
            <SignedIn>
              <Link
                href="/library"
                className="text-zinc-600 hover:text-zinc-900 dark:text-zinc-300 dark:hover:text-zinc-100"
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
