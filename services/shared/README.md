# services/shared

Cross-cutting Go primitives that are shared by every bounded context.

This module is intentionally narrow. New packages here require an ADR and a clear justification that the code cannot live inside a single context.

Allowed today:

- `auth/` — `Authenticator` port and adapters
- `storage/` — S3 adapter
- `eventbus/` — `EventBus` port (LISTEN/NOTIFY adapter for v0.1)
- `jobs/` — `JobBus` port (asynq adapter for v0.1)
- `obs/` — `slog` setup, OpenTelemetry helpers

Not allowed: `utils`, `helpers`, `common`. See `docs/adr/` for naming policy.
