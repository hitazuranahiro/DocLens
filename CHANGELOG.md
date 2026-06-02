# Changelog

All notable changes to DocLens are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/) with a slight twist:
each entry references the milestone (M-) it belongs to so the
mapping back to `.kiro/specs/doclens-v0.1` stays explicit.

## [Unreleased]

### Pending

- 7.5 — performance test for 1k-document libraries (target p95 < 500 ms).

## [0.1.0] — 2026-06-02

The first end-to-end version: a single-tenant document intelligence
platform that lets a signed-in user upload a PDF, watch it extract,
read the original alongside the Markdown, search across the library,
and delete documents — all running locally against a single
`docker compose` stack, all wired with sane defaults.

### Added

#### Foundation (M0–M2)

- Monorepo layout with bounded contexts under `services/` (ingestion,
  library, extraction, search, shared) and apps under `apps/`
  (`api`, `extraction-worker`, `web`).
- ADRs 0001–0012 capturing the v0.1 architectural decisions.
- OpenAPI 3.0 spec at `apps/api/openapi.yaml`; Go server stubs and
  TypeScript client are generated artifacts.
- Next.js 15 App Router + Clerk 6 sign-in flow, App / Marketing route
  groups, middleware-gated `/library` `/upload` `/search`.

#### Upload pipeline (M3)

- `POST /v1/uploads` with deterministic dedupe (200 on existing
  ownerId+sha256, 201 on fresh) and presigned PUT URLs capped at 5
  minutes (Property 7).
- `POST /v1/documents/{id}/finalize` validates the object landed and
  creates the canonical Library row.
- Web `UploadDropzone` computes SHA-256 client-side and finalizes on
  PUT success.
- Orphan-upload sweeper cron in the API.

#### Extraction (M4)

- `services/shared/jobs` JobBus port + asynq adapter; extraction worker
  consumes `extract.document` tasks.
- MarkItDown CLI adapter (Python) for PDF → Markdown extraction.
- `pdftoppm` thumbnail adapter; noop fallback when Poppler is absent.
- Confidence scoring (0–100) based on engine signals.
- `POST /v1/documents/{id}/retry` re-enqueues failed extractions.
- Multi-stage Dockerfile bundling Go + Python + MarkItDown + Tesseract
  - Poppler.

#### Library reader (M5)

- `GET /v1/documents` (cursor pagination), `GET /v1/documents/{id}`,
  `GET /v1/documents/{id}/markdown`, `/thumbnail`, `/raw`.
- Web `/library` list page + `/library/[id]` reader with PDF and
  Markdown side-by-side.

#### Live status (M6)

- Migration `0003_document_notify` fires `pg_notify('document_status', payload)`
  on inserts and on relevant column updates; payload is compact JSON
  capped under the 8 KB notify limit.
- `apps/api/internal/pubsub`: in-process Hub keyed by ownerID with
  per-subscriber 16-buffer channels (slow consumers are dropped, never
  block the listener); dedicated `pgx.Conn` Listener with exponential
  backoff reconnect.
- `GET /v1/documents/stream` SSE handler; 25 s `:keepalive` heartbeat.
- Same-origin Next Route Handler at `/api/documents/stream` injects
  the Clerk Bearer token EventSource cannot send itself.
- `useDocumentStream` hook + `DocumentListLive` client wrapper with a
  pure reducer (`applyEvent`) so the library list updates within ~1 s.

#### Search (M7)

- Migration `0004_search_documents` with a generated `tsvector` (title
  weight A, body weight B, english config) + GIN index.
- `services/search` bounded context with domain port + Postgres adapter.
- `services/shared/db.Querier` + `Transactor` for cross-context atomic
  writes; library + search repos rebind onto the same `pgx.Tx` for the
  ready step (Property 5).
- `GET /v1/search?q=…&cursor=…` with `websearch_to_tsquery`,
  `ts_headline` snippets, and `(rank, document_id)` cursor pagination.
- Web `/search` page: server-rendered first page, debounced client
  island that mirrors the query into the URL via `router.replace` in a
  `useTransition`. Snippets are sanitized through DOMPurify with a
  `<mark>`-only allowlist.

#### Delete (M8)

- `DELETE /v1/documents/{id}`: soft-deletes the row, removes the
  search index entry in the same tx, and asynchronously hard-deletes
  the underlying S3 objects (raw + artifacts).
- Idempotent: re-deleting returns 204 with `AlreadyDeleted=true`.
- In-flight extraction safely no-ops (Req 6.2).
- Web optimistic delete with rollback + alert banner on failure.

#### Quality, observability, ship (M9)

- Quickstart section in `README.md`: prereqs, `make bootstrap`,
  `make dev`, expected URLs, common make targets.
- DESIGN.md adapting Clerk's design language to DocLens; tokens are
  wired through `globals.css` → `tailwind.config.ts` → semantic class
  names on every component.
- `services/shared/observability/sentry`: env-gated Sentry shim used
  by API + worker. Empty `SENTRY_DSN` ⇒ no SDK init, no network.
- `services/shared/observability/otel`: env-gated OTLP/gRPC tracer for
  Go. Empty `OTEL_EXPORTER_OTLP_ENDPOINT` ⇒ no exporter built, no
  goroutines, no traffic. SDK still callable so spans are cheap nil
  effects.
- `@sentry/nextjs` and `@vercel/otel` wired through Next.js
  `instrumentation.ts`; both gated on env variables so local builds
  carry no telemetry.
- `apps/web/e2e/smoke.spec.ts`: Playwright spec walking
  health → me → upload → presigned PUT → finalize → poll-for-ready →
  fetch markdown → list → search → delete → list/search no-longer-find.
- Gated `E2E smoke` CI job in `.github/workflows/ci.yml`. Runs on push
  to `main` and on PRs labelled `e2e` so day-to-day PR feedback stays
  fast.
- `docs/operations/observability.md` documents both layers and the
  optional collector setup.

### Constraints met

- **Property 1** — every soft-delete cascades to artifacts in the
  same tx; orphan S3 objects are best-effort cleaned by the cleanup
  goroutine (sweeper landing in v0.2).
- **Property 2** — owner isolation is enforced at every storage port:
  every SQL query scopes by `owner_id`, search filters before tsquery
  match, the SSE hub is keyed by ownerID.
- **Property 3** — extraction is idempotent: re-running over an
  existing document re-uploads the artifacts (S3 overwrites by key)
  and upserts artifact rows + the search index.
- **Property 5** — extraction's ready step writes artifacts + search
  index + status flip in one Postgres tx. Delete writes soft-delete +
  search-row-removal in one tx. A failure rolls back both.
- **Property 6** — `(owner_id, sha256)` is unique among non-deleted
  rows; a re-upload after delete creates a fresh document.
- **Property 7** — presigned URLs are capped at 5 min in the storage
  adapter regardless of caller-supplied TTL.

### Known limitations

- Real Clerk sign-in is not exercised by the e2e smoke; the contract
  under test is the API token shape, which is the same in the real
  flow.
- The local `passthrough` extractor produces empty page/word counts
  because it doesn't parse PDFs; CI uses it to avoid bundling
  MarkItDown into the Linux runner.
- Performance target (Req 7.5: p95 < 500 ms on a 1k-document library)
  is tracked but not yet measured.
- OTel collector wiring exists in code; pointing it at a real
  collector is a deploy-time concern.

[0.1.0]: https://github.com/tomeku/doclens/releases/tag/v0.1.0
[Unreleased]: https://github.com/tomeku/doclens/compare/v0.1.0...HEAD
