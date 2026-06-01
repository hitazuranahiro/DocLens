# ADR 0010: Postgres FTS now, pgvector at v0.4

## Status

Accepted

## Date

2026-06-01

## Context

Search is a v0.1 feature ("find information instantly across large documents"). Two competing pressures:

1. Users expect "Google-quality" semantic search, especially when the product mentions AI prominently.
2. Operating a separate search system (Elastic, OpenSearch, Typesense, Weaviate) before product-market fit is a significant tax.

## Decision

- **v0.1:** Postgres full-text search via `tsvector` + `tsquery`, with a generated `tsvector` column over extracted text and document title. Multilingual config selectable per document. BM25-like ranking via `ts_rank_cd`.
- **v0.4:** Add `pgvector` for embedding-based semantic search alongside FTS. Hybrid retrieval (FTS + vector + reciprocal rank fusion) is the default once both exist.

Search lives in its own bounded context. Indexing is event-driven from `ExtractionCompleted`. The search API is `GET /search?q=...&library=...`.

If hybrid retrieval at scale exceeds Postgres's comfort zone (real-world signal: p95 above 500ms at modest volumes), we revisit by extracting Search to its own service backed by OpenSearch or a managed vector DB. The Search API contract does not change.

## Alternatives Considered

- **Elastic / OpenSearch from day one.** Overkill, JVM ops, separate cluster.
- **Typesense / Meilisearch.** Excellent products. We add them when FTS hurts, not before.
- **Weaviate / Qdrant only.** Skips lexical, which still wins for keyword queries.
- **SQLite FTS5.** Single-node only, not suitable for our deployment story.

## Consequences

**Positive**

- One system to operate.
- Index updates are part of the same transaction story as the rest of the data.
- Hybrid retrieval is a known good default once vectors arrive.

**Negative**

- Postgres FTS does not match a tuned Elastic index for ranking quality on long documents. We accept that for v0.1 and v0.2.
- `pgvector` index recall tuning (HNSW parameters) requires care; we will write a tuning doc when we cross that bridge.

**Neutral**

- Search has its own schema (`search.documents`, `search.embeddings`) separate from Library.
