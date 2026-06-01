// Protected layout. Middleware already enforces auth, so this layout
// trusts the session and only renders the chrome.
//
// Server Component: no client JS in the layout itself; UserButton is
// imported as a client component from Clerk and hydrated lazily.

import Link from "next/link";
import { UserButton } from "@clerk/nextjs";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col">
      <header className="border-b border-zinc-200 dark:border-zinc-800">
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-6">
          <Link href="/library" className="font-semibold tracking-tight">
            DocLens
          </Link>
          <nav className="flex items-center gap-6 text-sm">
            <Link
              href="/library"
              className="text-zinc-600 hover:text-zinc-900 dark:text-zinc-300 dark:hover:text-zinc-100"
            >
              Library
            </Link>
            <Link
              href="/upload"
              className="text-zinc-600 hover:text-zinc-900 dark:text-zinc-300 dark:hover:text-zinc-100"
            >
              Upload
            </Link>
            <Link
              href="/search"
              className="text-zinc-600 hover:text-zinc-900 dark:text-zinc-300 dark:hover:text-zinc-100"
            >
              Search
            </Link>
            <UserButton afterSignOutUrl="/" />
          </nav>
        </div>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-10">{children}</main>
    </div>
  );
}
