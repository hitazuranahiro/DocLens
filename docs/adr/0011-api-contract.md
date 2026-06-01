# ADR 0011: OpenAPI 3.1 contract-first

## Status

Accepted

## Date

2026-06-01

## Context

The web app and the Go API change together. We want:

- Compile-time safety on the TS side without hand-maintaining types.
- Reviewable contract changes that are obvious in PRs.
- A path to public API documentation in v0.4 without a rewrite.

## Decision

The API is **contract-first** with **OpenAPI 3.1**. The spec lives at `apps/api/openapi.yaml` and is the source of truth.

- Go: `oapi-codegen` generates server stubs (chi). Handlers implement the generated interfaces. Type drift is a build error.
- TypeScript: `openapi-typescript` generates types. `packages/api-client` wraps them with TanStack Query hooks.
- Contract changes happen in the spec first, then in code.

Public endpoints are documented and versioned with a `/v1/` prefix. Internal-only endpoints (admin, ops) use `/internal/` and are excluded from the public spec.

## Alternatives Considered

- **Code-first (Go structs → spec).** Spec ends up incomplete and matches the implementation by definition, defeating the point.
- **gRPC + protobuf.** Better contract, but the web client story is harder and we have no service-to-service traffic at v0.1.
- **GraphQL.** Powerful, mismatched for our resource-shaped product surface (documents, artifacts, jobs).
- **Hand-written types.** Drift, tears, suffering.

## Consequences

**Positive**

- One spec; two code-generation outputs. PR diffs make breaking changes visible.
- Future SDKs (Python, Go client lib) generate from the same spec.

**Negative**

- Codegen step in the build. We document the regen command and run it in CI.
- OpenAPI 3.1 tooling is younger than 3.0 in some Go libraries. We pin versions.

**Neutral**

- The spec doubles as API docs once we publish them.
