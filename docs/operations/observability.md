# Observability

DocLens v0.1 ships with two layers of observability — both env-gated
so local development and CI are inert by default.

## Sentry

Errors and panics from the Go API, the extraction worker, and the
Next.js web app are forwarded to Sentry when a DSN is configured.

| Surface                   | Env var                  |
| ------------------------- | ------------------------ |
| Go binaries (api, worker) | `SENTRY_DSN`             |
| Next.js (server + client) | `NEXT_PUBLIC_SENTRY_DSN` |

When the DSN is empty the SDK never initializes, no goroutines
start, no network calls fire. The SDK is built into the binary
either way; the on/off decision is purely runtime.

The Go shim lives at `services/shared/observability/sentry`.
The Next.js wiring uses `@sentry/nextjs` (`sentry.server.config.ts`,
`sentry.client.config.ts`, `sentry.edge.config.ts`,
`instrumentation.ts`).

Tag every event with the service name and release:

```
SENTRY_DSN=...
APP_RELEASE=$(git rev-parse --short HEAD)
GO_ENV=production
```

For Next.js the equivalent is `NEXT_PUBLIC_SENTRY_DSN` plus
`NEXT_PUBLIC_APP_RELEASE`. Public envs are the only way Next ships
values into the browser bundle; we use the same vars on the server
side for symmetry.

## OpenTelemetry tracing

The Go binaries and the Next.js server export OTLP/gRPC traces to a
collector when one is configured.

| Surface                   | Env var                       |
| ------------------------- | ----------------------------- |
| Go binaries (api, worker) | `OTEL_EXPORTER_OTLP_ENDPOINT` |
| Next.js (server)          | `OTEL_EXPORTER_OTLP_ENDPOINT` |

Empty endpoint disables tracing — no exporter is built, no batcher
runs, the global `TracerProvider` stays the SDK's no-op.

### Settings

| Variable                      | Default | Notes                         |
| ----------------------------- | ------- | ----------------------------- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | _unset_ | e.g. `otel-collector:4317`    |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true`  | Set `false` for TLS           |
| `OTEL_TRACES_SAMPLER_ARG`     | `0.1`   | Ratio sampler when in `[0,1)` |

Service name is hard-coded per binary (`doclens-api`,
`doclens-worker`, `doclens-web`) so traces from each process are
filterable in the collector. Resource attributes also include
`deployment.environment` (from `GO_ENV` / `NODE_ENV`) and
`service.version` (from `APP_RELEASE` / `NEXT_PUBLIC_APP_RELEASE`).

### Local collector

A no-op stack is fine for v0.1; if you want to inspect traces
locally, run an OpenTelemetry Collector container and point the
binaries at it:

```yaml
# docker-compose.dev.yml fragment
otel-collector:
  image: otel/opentelemetry-collector-contrib:latest
  command: ["--config=/etc/otel/config.yaml"]
  ports:
    - "4317:4317" # OTLP/gRPC
    - "4318:4318" # OTLP/HTTP
  volumes:
    - ./infra/otel-collector.yaml:/etc/otel/config.yaml
```

Then export:

```
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_EXPORTER_OTLP_INSECURE=true
```

The smoke test does not assert on trace export — its job is to
catch wiring breakages, and the no-op path is the unit tested
contract.

## Troubleshooting

- **No traces appearing**: confirm `OTEL_EXPORTER_OTLP_ENDPOINT`
  matches the collector's listener (gRPC port `4317` not the HTTP
  port `4318`). The Go shim uses gRPC unconditionally; mixing
  ports silently drops traffic.
- **TLS handshake errors**: set `OTEL_EXPORTER_OTLP_INSECURE=true`
  for in-cluster collectors that don't terminate TLS themselves.
- **Sentry events missing service tag**: check the Go binary set
  `ServiceName` in `sentry.Init`; `apps/api/cmd/api/main.go` does
  this once at boot.
