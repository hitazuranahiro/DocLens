// Package otel is the env-gated OpenTelemetry tracer setup used by
// every Go binary in the monorepo.
//
// It mirrors the Sentry shim:
//
//   shutdown := otel.Init(ctx, otel.Config{
//       Endpoint:    os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
//       ServiceName: "doclens-api",
//       Release:     version.Commit,
//       Logger:      logger,
//   })
//   defer shutdown(ctx, 5 * time.Second)
//
// When Endpoint is empty the package degrades to a no-op TracerProvider
// — no exporter is built, no goroutines start, no network calls fire.
// This is the contract for local development and CI.
//
// In production, point Endpoint at an OTLP/gRPC collector
// (e.g. http://otel-collector:4317). The exporter uses gRPC with the
// insecure transport unless OTEL_EXPORTER_OTLP_INSECURE=false; we
// keep the default insecure because the collector is expected to be
// in-cluster.
package otel

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

// Config bundles the inputs needed to start the tracer.
type Config struct {
	// Endpoint is the OTLP/gRPC collector address. Empty disables OTel.
	// Examples: "otel-collector.observability.svc:4317", "localhost:4317".
	Endpoint string
	// Insecure controls TLS to the collector. Defaults to true (the
	// most common deployment is collector-in-cluster). Set to false
	// when shipping over a public link.
	Insecure bool
	// ServiceName is required when Endpoint is set; tags every span.
	ServiceName string
	// Environment ("development", "staging", "production"). Tagged on
	// the resource so traces from each env are filterable.
	Environment string
	// Release tags the resource with the build commit / version.
	Release string
	// SampleRate in [0,1]. 0 → use parent-based always-sample.
	// In production you'll usually pin this lower (0.05–0.1).
	SampleRate float64
	// Logger receives the one bootstrap line. nil → slog.Default.
	Logger *slog.Logger
}

// Shutdown flushes pending spans and shuts down the exporter. Call
// from `defer` with a generous timeout (5–10s in production).
type Shutdown func(ctx context.Context, timeout time.Duration) error

// Init starts the tracer provider. Returns a no-op Shutdown when
// cfg.Endpoint is empty.
func Init(ctx context.Context, cfg Config) (Shutdown, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		logger.Info("otel: disabled (no OTEL_EXPORTER_OTLP_ENDPOINT)")
		return noopShutdown, nil
	}
	if cfg.ServiceName == "" {
		return nil, errors.New("otel: ServiceName is required when Endpoint is set")
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironment(orDefault(cfg.Environment, "development")),
			semconv.ServiceVersion(cfg.Release),
		),
	)
	if err != nil {
		return nil, err
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	} else {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}

	exp, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		return nil, err
	}

	sampler := sdktrace.ParentBased(sdktrace.AlwaysSample())
	if cfg.SampleRate > 0 && cfg.SampleRate < 1 {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)

	logger.Info("otel: enabled",
		"endpoint", cfg.Endpoint,
		"service", cfg.ServiceName,
		"env", cfg.Environment,
		"release", cfg.Release,
	)

	return func(shutdownCtx context.Context, timeout time.Duration) error {
		ctx, cancel := context.WithTimeout(shutdownCtx, timeout)
		defer cancel()
		return tp.Shutdown(ctx)
	}, nil
}

// Tracer returns a named tracer from the global provider. When OTel
// is disabled the returned tracer is the SDK's no-op tracer, so
// span creation is a cheap nil-effect.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func noopShutdown(_ context.Context, _ time.Duration) error { return nil }

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
