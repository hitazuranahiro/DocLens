# Local development stack

This directory contains the Docker Compose stack used by `make dev`.

## What runs

| Service           | Port        | Purpose                                    |
| ----------------- | ----------- | ------------------------------------------ |
| `postgres`        | 5432        | Application database                       |
| `redis`           | 6379        | asynq broker + cache                       |
| `minio`           | 9000 / 9001 | S3-compatible object storage + web console |
| `minio-bootstrap` | —           | One-shot job to create buckets             |

Future milestones add `api`, `extraction-worker`, and `web` services to this same file.

## Persistent data

State lives under `./data/` (gitignored). Wipe with:

```bash
make down
rm -rf infra/docker/data
```

## Default credentials (dev only)

- Postgres: `doclens / doclens` on database `doclens`
- MinIO: `doclens / doclens-dev-secret`, console at http://localhost:9001
- Redis: no auth

These are committed for local convenience. Production credentials must come from a secrets manager.

## Verifying the stack

```bash
make dev          # in one terminal
make ps           # in another — all services should report healthy/running
```

To poke at MinIO:

```bash
open http://localhost:9001
```

## Compose v2 vs legacy

The Makefile auto-detects whether `docker compose` (v2 plugin) or `docker-compose` (legacy) is on PATH. The compose file uses the v2 schema, which both honor.
