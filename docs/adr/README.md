# Architectural Decision Records

This directory captures the significant architectural decisions made on DocLens.

We use the lightweight format from Michael Nygard's _Documenting Architecture Decisions_.

## Format

Every ADR is a single Markdown file with these sections:

- **Status** — `Proposed`, `Accepted`, `Deprecated`, or `Superseded by ADR NNNN`
- **Date** — when the decision was accepted
- **Context** — the forces at play, the problem we are solving
- **Decision** — what we are doing, in one paragraph
- **Alternatives Considered** — what we rejected and why
- **Consequences** — positive, negative, and neutral effects

## Conventions

- Numbered with a four-digit zero-padded prefix: `0001`, `0002`, ...
- Filename: `NNNN-kebab-case-title.md`
- Never edit an accepted ADR. Supersede it with a new one and link both ways.
- Keep ADRs short. Two pages is plenty.

## Index

| #                                             | Title                                          | Status             |
| --------------------------------------------- | ---------------------------------------------- | ------------------ |
| [0001](0001-record-architecture-decisions.md) | Record architecture decisions                  | Accepted           |
| [0002](0002-monorepo-structure.md)            | Monorepo with Turborepo and Go workspaces      | Accepted           |
| [0003](0003-service-boundaries.md)            | Service boundaries aligned to bounded contexts | Accepted           |
| [0004](0004-pdf-extraction-toolchain.md)      | PDF extraction via Poppler and MuPDF           | Superseded by 0012 |
| [0005](0005-background-jobs-asynq.md)         | Background jobs with asynq on Redis            | Accepted           |
| [0006](0006-postgres-sqlc-pgx.md)             | Postgres + sqlc + pgx, no ORM                  | Accepted           |
| [0007](0007-object-storage.md)                | S3-compatible object storage                   | Accepted           |
| [0008](0008-auth-strategy.md)                 | Auth via Clerk for v0.1, behind a port         | Accepted           |
| [0009](0009-event-bus-evolution.md)           | Events via Postgres LISTEN/NOTIFY, NATS later  | Accepted           |
| [0010](0010-search-strategy.md)               | Postgres FTS now, pgvector at v0.4             | Accepted           |
| [0011](0011-api-contract.md)                  | OpenAPI 3.1 contract-first                     | Accepted           |
| [0012](0012-extraction-engine-markitdown.md)  | Microsoft MarkItDown as the extraction engine  | Accepted           |

## Proposing a new ADR

1. Copy `_template.md` to `NNNN-your-title.md`.
2. Open with status `Proposed`.
3. Open a PR. Discuss in review.
4. On merge, status flips to `Accepted` and the index is updated.
