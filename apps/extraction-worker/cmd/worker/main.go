// Command worker is the DocLens extraction worker.
//
// It connects to Redis, registers task handlers, and runs the asynq
// server until SIGINT/SIGTERM. PR 2 wires real DB and S3 dependencies;
// PR 1 ships a logging stub so the queue plumbing is verifiable.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"

	"github.com/tomeku/doclens/apps/extraction-worker/internal/config"
	"github.com/tomeku/doclens/apps/extraction-worker/internal/handlers"
	"github.com/tomeku/doclens/services/extraction/adapters/markitdown"
	"github.com/tomeku/doclens/services/extraction/adapters/passthrough"
	"github.com/tomeku/doclens/services/extraction/domain"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		logger.Error("invalid redis url", "err", err, "url", cfg.RedisURL)
		os.Exit(1)
	}

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: cfg.Concurrency,
		Queues:      cfg.Queues,
		// Errors here are operational. Default behavior is fine for
		// v0.1; we plug Sentry in M9.
		ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, t *asynq.Task, err error) {
			logger.Error("asynq task error",
				"type", t.Type(),
				"err", err,
			)
		}),
	})

	mux := asynq.NewServeMux()
	handlers.Register(mux, handlers.NewExtractHandler(logger, buildExtractor(cfg, logger)))

	go func() {
		logger.Info("worker starting",
			"env", cfg.Env,
			"queues", cfg.Queues,
			"concurrency", cfg.Concurrency,
		)
		if err := srv.Run(mux); err != nil {
			logger.Error("asynq server stopped with error", "err", err)
			os.Exit(1)
		}
	}()

	// Wait for a signal, then ask asynq to drain in-flight tasks.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logger.Info("shutting down worker")
	srv.Shutdown()
	srv.Stop()
}

// buildExtractor returns the production MarkItDown adapter when the
// CLI is configured, falling back to the passthrough fake otherwise.
//
// PR 1 keeps the fallback lenient so a contributor can run the
// worker against a Redis-only stack without installing Python yet.
// PR 2 tightens this to fail closed in production.
func buildExtractor(cfg config.Config, logger *slog.Logger) domain.Extractor {
	if cfg.MarkItDownBin == "passthrough" {
		logger.Warn("extractor: passthrough (development only)")
		return passthrough.New()
	}
	logger.Info("extractor: markitdown",
		"bin", cfg.MarkItDownBin,
		"timeout", cfg.MarkItDownTimeout,
	)
	return markitdown.New(markitdown.Config{Bin: cfg.MarkItDownBin})
}
