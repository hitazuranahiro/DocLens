# ADR 0006: Postgres + sqlc + pgx, no ORM

## Status

Accepted

## Date

2026-06-01

## Context

DocLens stores document metadata, extraction state, search indexes, and (later) embeddings. PostgreSQL covers all of these well: JSONB for flexible metadata, full-text search via `tsvector`, vector similarity via `pgvector`.

The decision is how to talk to it from Go.

## Decision

- **Driver:** `jackc/pgx/v5`. The de facto Go Postgres driver. Connection pool, prepared statements, native types.
- **Queries:** `sqlc`. Write SQL, generate type-safe Go. No reflection, no runtime mapping, no DSL to learn.
- **Migrations:** `golang-migrate/migrate` with `up.sql` / `down.sql` pairs.
- **Schema split:** One Postgres schema per bounded context (`ingestion`, `extraction`, `library`, `search`, `intelligence`). Cross-schema queries are forbidden by convention and enforced by `sqlc` config.

## Alternatives Considered

- **GORM.** Reflection at runtime, magic methods, surprises in transaction semantics.
- **ent.** Powerful, but the schema-as-Go approach inverts what we want. We want SQL to be the source of truth.
- **squirrel** or **goqu**. Better than ORMs but still produce stringly-typed queries.
- **Plain `database/sql` + scan helpers.** Fine but loses the type safety we get from `sqlc`.

## Consequences

**Positive**

- SQL is the source of truth. Reviewable in PRs.
- `sqlc` produces zero-allocation types and clear method names.
- Schema-per-context catches accidental cross-context coupling at compile time.

**Negative**

- `sqlc` requires regeneration after schema changes. CI runs `sqlc diff` to enforce drift-free.
- No automatic migrations from Go struct changes; we write SQL.

**Neutral**

- Switching drivers is unlikely; switching to a different generator is possible because handlers depend on repository interfaces.
