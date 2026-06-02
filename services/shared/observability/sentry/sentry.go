// Package sentry is a thin shim over the official Sentry Go SDK that
// is safe to import from every binary in the monorepo. When
// SENTRY_DSN is empty the package degrades to a no-op so we don't
// drag the SDK's network calls into local development or tests.
//
// Usage:
//
//   shutdown := sentry.Init(sentry.Config{
//       DSN:         os.Getenv("SENTRY_DSN"),
//       Environment: cfg.Env,
//       Release:     version.Commit,
//       ServiceName: "doclens-api",
//   })
//   defer shutdown(2 * time.Second)
//
// Errors and panics are best-effort: we capture and continue. The
// caller decides whether observability is critical (it isn't, in
// v0.1).
package sentry

import (
	"log/slog"
	"time"

	sentrygo "github.com/getsentry/sentry-go"
)

// Config bundles the inputs needed to start Sentry.
type Config struct {
	// DSN is the Sentry project DSN. Empty disables Sentry entirely.
	DSN string
	// Environment is the runtime tag ("development", "staging",
	// "production"). Defaults to "development" when empty.
	Environment string
	// Release is the version/commit string Sentry uses to tie events
	// to a deploy. Empty is fine.
	Release string
	// ServiceName is attached as a tag so events from api / worker /
	// web are easy to filter.
	ServiceName string
	// SampleRate is the trace/error sample rate in [0,1]. Defaults to
	// 1.0 (capture everything) when zero.
	SampleRate float64
	// Logger is used for the one bootstrap log line. nil → slog.Default.
	Logger *slog.Logger
}

// Shutdown is the cleanup function returned by Init. Pass to defer.
type Shutdown func(timeout time.Duration)

// Init starts the global Sentry hub. Returns a no-op Shutdown when
// Config.DSN is empty.
func Init(cfg Config) Shutdown {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.DSN == "" {
		logger.Info("sentry: disabled (no DSN)")
		return noopShutdown
	}
	env := cfg.Environment
	if env == "" {
		env = "development"
	}
	rate := cfg.SampleRate
	if rate <= 0 {
		rate = 1.0
	}
	if err := sentrygo.Init(sentrygo.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      env,
		Release:          cfg.Release,
		TracesSampleRate: rate,
		// AttachStacktrace makes captured errors and panics easier to
		// triage; the cost is negligible for our volume.
		AttachStacktrace: true,
	}); err != nil {
		logger.Warn("sentry: init failed; disabled", "err", err)
		return noopShutdown
	}
	if cfg.ServiceName != "" {
		sentrygo.ConfigureScope(func(scope *sentrygo.Scope) {
			scope.SetTag("service", cfg.ServiceName)
		})
	}
	logger.Info("sentry: enabled",
		"environment", env,
		"release", cfg.Release,
		"service", cfg.ServiceName,
	)
	return func(timeout time.Duration) {
		sentrygo.Flush(timeout)
	}
}

// CaptureError forwards an error to Sentry. Safe to call even when
// Init was a no-op (the underlying SDK swallows pre-init calls).
func CaptureError(err error) {
	if err == nil {
		return
	}
	sentrygo.CaptureException(err)
}

func noopShutdown(time.Duration) {}
