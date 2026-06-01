# apps/extraction-worker

The DocLens extraction worker. Runs as a long-lived process consuming
the `extract.document` asynq task and producing Markdown + metadata
artifacts.

## Layout

```
extraction-worker/
├── cmd/worker/main.go          # entrypoint, signal handling
└── internal/
    ├── config/                 # env loader (REDIS_URL, MARKITDOWN_BIN, ...)
    └── handlers/               # asynq task handlers
```

## Environment

| Var                          | Purpose                                              | Default                    |
| ---------------------------- | ---------------------------------------------------- | -------------------------- |
| `REDIS_URL`                  | asynq broker DSN.                                    | `redis://localhost:6379/0` |
| `WORKER_CONCURRENCY`         | parallel task workers per process                    | `4`                        |
| `MARKITDOWN_BIN`             | extractor binary; `passthrough` enables the dev fake | `markitdown`               |
| `MARKITDOWN_TIMEOUT`         | per-attempt subprocess deadline                      | `5m`                       |
| `EXTRACTION_ENABLED_FORMATS` | comma-separated MIME allow-list                      | `application/pdf`          |

## Status (M4 PR 1)

This PR ships:

- The worker binary skeleton.
- The `extract.document` task type registration.
- A logging-only handler stub so the queue plumbing can be verified.

PR 2 replaces the stub body with the full pipeline (download → extract → artifacts → status=ready) and adds the production Dockerfile (Go + Python 3.12 + MarkItDown).
