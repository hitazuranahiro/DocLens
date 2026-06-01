# ADR 0007: S3-compatible object storage

## Status

Accepted

## Date

2026-06-01

## Context

DocLens stores raw uploaded files plus derived artifacts: extracted Markdown, page images, table CSVs, thumbnails. Documents can be hundreds of megabytes. Storing bytes in Postgres is wrong. We need durable, cheap, streaming-friendly object storage that is the same API in dev and prod.

## Decision

Use the S3 API everywhere.

- **Dev:** MinIO via `docker-compose`.
- **Prod self-host:** MinIO or operator-provided S3-compatible store.
- **Hosted prod:** Cloudflare R2 first (no egress fees, generous free tier), AWS S3 as a tested fallback.

We use the official `aws-sdk-go-v2` with a configurable endpoint, not a vendor-specific SDK.

Buckets:

- `doclens-raw` — original uploads, write-once
- `doclens-artifacts` — derived files
- `doclens-public` — thumbnails fronted by a CDN

All uploads go directly browser → object store via presigned PUT URLs. The API never proxies bytes.

## Alternatives Considered

- **Postgres `bytea`.** Trivial but unbounded growth, slow to vacuum, kills replication.
- **Filesystem on the API server.** No durability, no horizontal scale.
- **Vendor-locked SDKs (R2 SDK, GCS SDK).** Reduces portability without meaningful gain.

## Consequences

**Positive**

- One adapter, many backends. Self-host and managed both work.
- API stays stateless for uploads.
- Object lifecycle rules handle retention without app code.

**Negative**

- We must be careful with presigned URL expiry and CORS in dev.
- Eventual consistency for listing on some backends (not S3 anymore, but worth noting for forks).

**Neutral**

- All bucket names are config; no hardcoded names anywhere in the codebase.
