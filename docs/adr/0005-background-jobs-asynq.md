# ADR 0005: Background jobs with asynq on Redis

## Status

Accepted

## Date

2026-06-01

## Context

Extraction is the prototypical background job: minutes long, retryable, idempotent, and benefits from priority queues (a 2-page note should not wait behind a 400-page manual). We will also need scheduled jobs for cleanup and re-indexing.

We want a queue that is idiomatic Go, observable, and runnable in `docker-compose` without a fight.

## Decision

Use `hibiken/asynq` with Redis as the broker. Each service owns its task types; workers are deployed per service. The asynq web UI is available in dev under `/queues`.

Jobs are typed with a `Payload` struct serialized to JSON. Handlers live in `services/<context>/internal/jobs/`. Retries are configured per task type with exponential backoff.

## Alternatives Considered

- **River.** Postgres-backed, newer. Compelling because it removes Redis. Reconsider when River reaches API stability and we have a migration story for asynq's UI.
- **Channel pool inside the API process.** No durability, no replay, NIH.
- **Temporal.** Powerful, but an entire control plane to operate. Overkill for v0.1.
- **Sidekiq via Ruby sidecar.** Cross-language. No.

## Consequences

**Positive**

- Retries, scheduling, priorities, deduplication out of the box.
- Asynq dashboard makes ops investigation cheap.
- Same Redis instance is reused for caching.

**Negative**

- Redis becomes a hard dependency for any code path that enqueues work.
- We must keep handlers idempotent. Documented in `docs/architecture/jobs.md`.

**Neutral**

- Migration to River later is plausible since handlers depend on `domain.JobBus`, not on asynq directly.
