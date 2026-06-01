# services/extraction

Extraction bounded context. Owns the conversion of raw uploaded files
into Markdown plus metadata.

Per [ADR 0012](../../docs/adr/0012-extraction-engine-markitdown.md) the
v0.1 engine is Microsoft MarkItDown invoked via subprocess. The
`Extractor` port keeps that decision contained — adding a Poppler
fallback or an HTTP sidecar mode swaps the adapter only.

## Layout

```
extraction/
├── domain/                       # Extractor port + errors
└── adapters/
    ├── markitdown/               # production adapter, shells out to CLI
    └── passthrough/              # test/dev adapter; treats input as Markdown
```

The worker binary that consumes the port lives in `apps/extraction-worker`.

## Tasks

- M4 PR 1: port + adapters + worker skeleton.
- M4 PR 2: real handler that downloads bytes, writes artifacts, updates the document row, and computes the confidence score.
