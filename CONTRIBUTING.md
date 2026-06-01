# Contributing to DocLens

Thanks for considering a contribution. This guide gets you from `git clone` to a passing PR.

## Prerequisites

- Node.js 20+ and pnpm 9+
- Go 1.23+
- Docker (with Compose v2; legacy `docker-compose` is also supported)
- Make

## Quickstart

```bash
git clone https://github.com/tomeku/doclens.git
cd doclens
make bootstrap   # installs JS deps, syncs Go modules
make dev         # brings up Postgres, Redis, MinIO, api, worker, web
```

`make dev` runs the full local stack via `infra/docker/docker-compose.dev.yml`. The web app listens on http://localhost:3000 and the API on http://localhost:8080.

## Workflow

1. Fork and create a feature branch off `main`.
2. Read the relevant ADR(s) in `docs/adr/` before changing architecture.
3. If your change touches `apps/api/openapi.yaml`, run `make gen` to regenerate the Go server stubs and TS client in the same PR.
4. Add or update tests. Run `make test`.
5. Run `make lint` before pushing.
6. Open a PR. Reference the spec task you're closing: `Closes #N (Task 3.4)`.

## Project Structure

See [PROJECT.md](PROJECT.md). In short:

- `apps/web` — Next.js
- `apps/api` — Go HTTP gateway
- `apps/extraction-worker` — Go worker that wraps MarkItDown
- `services/<context>` — Bounded-context Go modules
- `packages/*` — Shared TS packages
- `infra/docker` — Local dev compose stack
- `docs/adr` — Architectural Decision Records
- `docs/specs` — Active feature specs

## Coding standards

- TypeScript: strict mode, no `any` without a comment justifying it.
- Go: `gofmt` + `golangci-lint`. No new `interface{}`; use `any` only when generics aren't enough.
- SQL: lowercase keywords, snake_case columns, one schema per bounded context.
- No new top-level packages without an ADR. No `utils`, `helpers`, `common`, `shared` outside the explicitly-allowed `services/shared`.

## Commit messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(library): cursor pagination for /v1/documents
fix(extraction): retry on S3 5xx
docs(adr): supersede 0004 with 0012
```

## Reporting issues

Open a GitHub issue with reproduction steps, expected behavior, and observed behavior. For security issues, email security@tomeku.com instead of opening a public issue.

## License

By contributing, you agree that your contributions will be licensed under the project's MIT License.
