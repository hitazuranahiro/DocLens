import Link from "next/link";

import { UploadDropzone } from "@/components/documents/UploadDropzone";

export default function UploadPage() {
  return (
    <div className="space-y-8">
      <header className="space-y-2">
        <h1 className="text-heading text-text-strong">Upload</h1>
        <p className="text-body text-muted">
          Drop a PDF below. We hash it locally, presign an upload, and queue extraction. Files
          already in your library are detected and skipped automatically.
        </p>
      </header>

      <UploadDropzone />

      <p className="text-caption text-muted">
        Looking for something you already uploaded?{" "}
        <Link href="/library" className="text-brand underline-offset-4 hover:underline">
          Open your library
        </Link>
        .
      </p>
    </div>
  );
}
