# ADR 0012: Microsoft MarkItDown as the extraction engine

## Status

Accepted (supersedes [ADR 0004](0004-pdf-extraction-toolchain.md))

## Date

2026-06-01

## Context

DocLens converts user-uploaded documents into clean Markdown plus metadata. Our roadmap covers PDFs in v0.1 and Office, EPUB, HTML, and image-based formats by v0.2. ADR 0004 picked Poppler + MuPDF, which is excellent for PDF but commits us to writing a new adapter for every additional format.

Microsoft released [`markitdown`](https://github.com/microsoft/markitdown) under MIT license. It is a single Python library that converts:

- PDF (via `pdfminer.six`)
- Word (`.docx` via `mammoth`)
- PowerPoint (`.pptx`)
- Excel (`.xlsx`, `.xls`)
- HTML
- EPUB
- Images (EXIF + optional OCR via `pytesseract`, optional LLM captioning)
- Audio (EXIF + optional speech transcription)
- ZIP archives (recurses into contents)
- CSV, JSON, XML, plain text
- YouTube URLs (transcript)

The output is Markdown — exactly what DocLens already standardizes on as its intermediate representation.

The downside: MarkItDown is Python. Our extraction worker is Go.

## Decision

Use **Microsoft MarkItDown** as the v0.1 extraction engine.

Integration model: the Go extraction worker shells out to the `markitdown` CLI as a subprocess. The worker container ships with Python 3.12, `markitdown[all]`, Tesseract, FFmpeg, and the system packages MarkItDown needs.

The `Extractor` port in `services/extraction/domain/` keeps the same shape as before:

```go
type Extractor interface {
    Extract(ctx context.Context, src io.Reader, hint MimeHint) (*Result, error)
}

type Result struct {
    Markdown   string
    Metadata   map[string]any
    Pages      int
    Confidence float32
    Warnings   []string
}
```

The single v0.1 adapter (`adapters/markitdown/`) wraps the CLI. If we later need lower latency or want to skip the subprocess hop, we expose MarkItDown over HTTP as a sidecar; the port does not change.

We keep MarkItDown's optional LLM image captioning **off** by default to avoid surprise OpenAI calls. Users opt in per-document or via a workspace setting (post-v0.1).

## Alternatives Considered

- **Poppler + MuPDF only** (the prior decision). Best PDF fidelity. Wrong shape for our roadmap. Forces a separate adapter per format starting v0.2.
- **Apache Tika.** Comparable format coverage. JVM container, slower cold start, more memory, weaker Markdown output (Tika emits XHTML; we would need a converter step).
- **Unstructured.io.** Excellent for tables and chunking, but heavier dependency tree, parts of the project use a Source-Available license, and the API leans toward RAG pipelines rather than clean Markdown.
- **Pure-Go PDF library (`unidoc`, `ledongthuc/pdf`).** Single language, but PDF only and weaker layout fidelity than `pdfminer.six`.
- **Python worker end-to-end.** Tempting because MarkItDown is native Python, but `asynq` is Go-only and we already have the rest of the stack in Go. Subprocess shell-out keeps the boundary clean.
- **Sidecar HTTP service from day one.** More moving parts than v0.1 needs. Defer until profiling shows subprocess overhead matters.

## Consequences

**Positive**

- One library covers the entire v0.1 + v0.2 format roadmap.
- Markdown is the native output. No format converter step.
- Microsoft maintains the parsers, including security patches.
- OCR and audio transcription become flag-flips, not new modules.
- ZIP-recursion gives us a credible "upload a folder of papers" story for free.

**Negative**

- Worker container has Python alongside Go. Image is larger (~150 MB more); we offset by using a slim Python base.
- PDF layout fidelity is good but not Poppler-grade. We document the limitation and may add a Poppler fallback for high-fidelity research-paper mode in v0.2.
- Subprocess overhead per document. Acceptable for v0.1 latency targets (p50 < 30 s for ≤ 50 pages); revisit if profiling disagrees.
- We track MarkItDown releases. We pin the version in `pyproject.toml`-style lockfile inside the worker image.

**Neutral**

- The `Extractor` port is unchanged from ADR 0004. Swapping engines is contained.
- Confidence scoring is now derived from MarkItDown warnings + heuristics (page count, word density, error events) rather than tool-specific exit codes.
