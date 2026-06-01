// Protected layout. Middleware enforces auth so this layout trusts
// the session and only renders chrome.

import Link from "next/link";
import { UserButton } from "@clerk/nextjs";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen flex-col bg-bg">
      <header className="border-b border-border">
        <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-6">
          <Link href="/library" className="text-title text-text-strong tracking-tight">
            DocLens
          </Link>
          <nav className="flex items-center gap-6 text-label">
            <NavLink href="/library">Library</NavLink>
            <NavLink href="/upload">Upload</NavLink>
            <NavLink href="/search">Search</NavLink>
            <UserButton afterSignOutUrl="/" />
          </nav>
        </div>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-6 py-12">{children}</main>
    </div>
  );
}

function NavLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <Link href={href} className="text-muted transition-colors duration-base hover:text-text-strong">
      {children}
    </Link>
  );
}
