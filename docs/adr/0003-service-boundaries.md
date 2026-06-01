# ADR 0003: Service boundaries aligned to bounded contexts

## Status

Accepted

## Date

2026-06-01

## Context

DocLens has five bounded contexts: Ingestion, Extraction, Library, Search, Intelligence. Two extremes are tempting and both are wrong for v0.1:

1. **One Go binary forever.** Easy to start. Will not survive extraction's CPU profile, which can pin a worker for minutes.
2. **A microservice per context from day one.** Premature. We do not yet know the real load profile or where the boundaries leak.

We need a layout that gives us context separation without operational sprawl, and that makes future extraction easy.

## Decision

Each bounded context lives in its own Go module under `services/`. Public APIs are exposed via interfaces in a `domain/` package per service. The HTTP gateway in `apps/api/` imports those modules and routes requests to in-process handlers.

Workers are separate binaries that import the same modules and consume jobs from the queue (see ADR 0005).

When a context needs to be extracted to its own deployable, its `domain/` package becomes the gRPC or HTTP contract; nothing inside the context changes.

## Alternatives Considered

- **Monolith with package boundaries only.** Easy to violate. We have seen `helpers.go` cross-cut everything within six months.
- **Microservices day one.** Adds latency, deploy surface, and an event bus we do not need yet.
- **Hexagonal monolith with one binary, no modules.** Single `go.mod` makes it tempting to import `extraction/internal/...` from `library/`. Separate modules make that a build error.

## Consequences

**Positive**

- One binary per role today (`api`, `extraction-worker`), N tomorrow.
- Cross-context calls are explicit and reviewable.
- Each context has its own `migrations/` directory and Postgres schema.

**Negative**

- Slightly more friction to share types: shared types live in a `services/shared/` module by deliberate exception.
- Multiple `go.mod` files require `go.work` discipline.

**Neutral**

- The bounded-context names are the ubiquitous language. Renaming a context is an architectural event, not a refactor.
