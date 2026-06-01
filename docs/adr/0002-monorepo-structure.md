# ADR 0002: Monorepo with Turborepo and Go workspaces

## Status

Accepted

## Date

2026-06-01

## Context

DocLens has a Next.js web app, a Go HTTP API, and several Go services that share types and protocols. We need a repository layout that:

- Lets a single change span web + api when an endpoint changes shape.
- Generates TypeScript types from the API contract automatically.
- Keeps Go modules independently buildable while sharing dev tooling.
- Stays cheap to operate in CI for a small team.

## Decision

Single Git repository organized as:

```
apps/        # Deployable user-facing programs (web, api)
services/    # Bounded-context Go modules
packages/    # Shared TS packages
infra/       # Docker, K8s, migrations
docs/        # Architecture, ADRs
```

JavaScript/TypeScript pipelines are managed by Turborepo. Go modules are stitched with `go.work`. Every commit runs only the affected pipelines via Turborepo's remote cache and `go test ./...` scoped by changed paths.

## Alternatives Considered

- **Polyrepo.** High coordination cost early. A schema change in `api` would be three PRs across three repos. Move to polyrepo when a service genuinely needs an independent release cadence.
- **Nx.** Powerful but more moving parts than we need for two languages.
- **Bazel.** Overkill for the team size and language count.

## Consequences

**Positive**

- One PR can change web + api + types together.
- Shared `eslint-config`, `tsconfig`, `prettier` config.
- Onboarding is `git clone && make dev`.

**Negative**

- CI must scope work; a naive `go test ./...` from the root gets slow as services multiply.
- `go.work` files should not be committed for libraries published outside the repo. We accept that constraint by keeping all Go code internal.

**Neutral**

- We can extract a service to its own repo later. The seams are the bounded-context boundaries, not the file layout.
