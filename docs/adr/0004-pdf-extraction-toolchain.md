# ADR 0004: PDF extraction via Poppler and MuPDF

## Status

**Superseded by [ADR 0012](0012-extraction-engine-markitdown.md)** on 2026-06-01.

The original decision below is preserved for historical context. Implementation should follow ADR 0012.

## Date

2026-06-01

## Context

PDF is the priority format. PDF extraction is hard: layout-aware text, tables, embedded images, encrypted files, scanned (image-only) PDFs. We needed a baseline in v0.1 that handled "normal" digital PDFs reliably.

The Go ecosystem has PDF readers (`unidoc`, `pdfcpu`, `ledongthuc/pdf`) but layout fidelity and table extraction are uneven, and licensing varies.

## Decision

v0.1 shells out to mature C tools:

- `pdftotext -layout` (Poppler) for text with reading order.
- `pdfinfo` (Poppler) for metadata.
- `pdfimages` (Poppler) for embedded images.
- `mutool draw` (MuPDF) for page rendering when we need a thumbnail.

These run inside the extraction worker container. The Go code wraps them behind an `Extractor` interface in `services/extraction/domain/extractor.go`.

## Why Superseded

Microsoft's MarkItDown (released under MIT license) covers PDF, Office, EPUB, HTML, images, and audio in a single Python library that emits Markdown directly. Choosing Poppler-only would have forced us to write five more format adapters by v0.2. The cost of adding Python to the worker container is smaller than the cost of those adapters and their long-term maintenance.

See [ADR 0012](0012-extraction-engine-markitdown.md) for the replacement decision.

## Alternatives Considered (Original)

- **Pure-Go PDF library.** Layout fidelity poor for academic papers and reports.
- **Apache Tika.** Heavy JVM, large container image, brittle table extraction.
- **Commercial APIs (AWS Textract, Google Document AI).** Closed source, per-page cost, kills self-hostability.
- **Build our own.** No.
