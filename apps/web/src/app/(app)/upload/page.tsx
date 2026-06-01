// Upload page. Renders the dropzone in the protected (app) layout.
// The dropzone itself is a client component because it owns local
// state and uses Clerk's getToken; the page wrapper stays a Server
// Component so navigation prefetches stay cheap.

import Link from "next/link";

import { UploadDropzone } from "@/components/documents/UploadDropzone";

export default function UploadPage() {
  return (
    <div className="space-y-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">Upload</h1>
        <p className="text-sm text-zinc-600 dark:text-zinc-300">
          Drop a PDF below. We hash it locally, presign an upload, and queue extraction. Files
          already in your library are detected and skipped automatically.
        </p>
      </header>

      <UploadDropzone />

      <p className="text-xs text-zinc-500 dark:text-zinc-400">
        Looking for something you already uploaded?{" "}
        <Link href="/library" className="underline">
          Open your library
        </Link>
        .
      </p>
    </div>
  );
}
