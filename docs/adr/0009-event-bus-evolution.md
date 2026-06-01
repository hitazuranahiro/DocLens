# ADR 0009: Events via Postgres LISTEN/NOTIFY, NATS later

## Status

Accepted

## Date

2026-06-01

## Context

Bounded contexts need to react to each other's events: "DocumentIngested" → Extraction enqueues a job; "ExtractionCompleted" → Search reindexes and Library updates status. Two questions:

1. Is the message bus durable, or are jobs the durable mechanism?
2. Do we run a separate broker on day one?

## Decision

For v0.1, durability lives in **the job queue (asynq)**, not the event bus. Cross-context notifications use **Postgres LISTEN/NOTIFY** as a fanout mechanism for in-memory subscribers (e.g. WebSocket push to the web client). Workflow events that must not be lost are modeled as enqueued jobs, not pub/sub messages.

When v0.3+ introduces multiple consumers per event with replay needs (e.g. embeddings, search, analytics all consuming `ExtractionCompleted`), we move to **NATS JetStream**. The migration is contained: producers call `eventbus.Publish(ctx, event)` and consumers call `eventbus.Subscribe(...)`.

## Alternatives Considered

- **Kafka day one.** Operational cost for a feature we do not need yet.
- **Redis Streams.** Reasonable but we want NATS's subject hierarchy and ergonomic Go client when we get there.
- **Outbox pattern with polling.** Overkill given asynq already provides durable retries.

## Consequences

**Positive**

- Zero new infrastructure for v0.1. Postgres + Redis is the entire stateful surface.
- Real-time UI updates land cheaply via LISTEN/NOTIFY.

**Negative**

- Cross-context coupling shows up as direct job enqueueing rather than published events. Reviewers must keep this honest.
- LISTEN/NOTIFY is fire-and-forget; do not depend on it for correctness.

**Neutral**

- The `EventBus` interface is the same in both eras. Adapters change.
