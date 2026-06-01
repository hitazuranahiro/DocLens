# Database migrations

DocLens uses [`golang-migrate/migrate`](https://github.com/golang-migrate/migrate) with `up.sql` / `down.sql` pairs (per ADR 0006).

## Conventions

- Forward-only in CI. `down.sql` files are for local development.
- One migration covers all schemas in a release. Each statement is namespaced (`library.documents`, `ingestion.uploads`, etc.) so ownership stays clear.
- File name: `NNNN_description.{up|down}.sql` where `NNNN` is a four-digit sequential number.
- Migrations run inside a `BEGIN; ... COMMIT;` block so a failure leaves no partial state.

## Running locally

```bash
# From the repo root, with the dev compose stack up:
make migrate
```

`make migrate` invokes the `golang-migrate` CLI against `DATABASE_URL`
(default `postgres://doclens:doclens@localhost:5432/doclens?sslmode=disable`).

## Schema layout

| Schema       | Owner context | Tables                   |
| ------------ | ------------- | ------------------------ |
| `ingestion`  | Ingestion     | `uploads`                |
| `library`    | Library       | `documents`, `artifacts` |
| `extraction` | Extraction    | (M4)                     |
| `search`     | Search        | (M7)                     |

Cross-schema joins are not allowed (ADR 0006). Foreign keys across schemas are fine.
