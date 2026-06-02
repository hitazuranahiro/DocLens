// Command worker is the DocLens extraction worker.
//
// It connects to Redis, registers task handlers, and runs the asynq
// server until SIGINT/SIGTERM. The extract.document handler delegates
// to extraction/app.Service which performs the full pipeline:
// download → run extractor → upload artifacts → flip status.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tomeku/doclens/apps/extraction-worker/internal/config"
	"github.com/tomeku/doclens/apps/extraction-worker/internal/handlers"
	"github.com/tomeku/doclens/apps/extraction-worker/internal/readytx"
	extractionapp "github.com/tomeku/doclens/services/extraction/app"
	"github.com/tomeku/doclens/services/extraction/adapters/markitdown"
	"github.com/tomeku/doclens/services/extraction/adapters/noopthumbnailer"
	"github.com/tomeku/doclens/services/extraction/adapters/passthrough"
	"github.com/tomeku/doclens/services/extraction/adapters/pdftoppm"
	"github.com/tomeku/doclens/services/extraction/domain"
	libpg "github.com/tomeku/doclens/services/library/adapters/postgres"
	"github.com/tomeku/doclens/services/shared/observability/sentry"
	storages3 "github.com/tomeku/doclens/services/shared/storage/s3"
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

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sentryShutdown := sentry.Init(sentry.Config{
		DSN:         cfg.SentryDSN,
		Environment: cfg.Env,
		Release:     cfg.Release,
		ServiceName: "doclens-worker",
		Logger:      logger,
	})
	defer sentryShutdown(2 * time.Second)

	// Best-effort init of Postgres + S3. If either is down we still
	// boot so the queue plumbing can be debugged in dev; the
	// handler refuses to run extractions until both are up.
	pool, poolCleanup := mustOrSkipPool(rootCtx, cfg, logger)
	defer poolCleanup()

	store, err := storages3.New(rootCtx, storages3.Config{
		Endpoint:        cfg.S3Endpoint,
		Region:          cfg.S3Region,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
		UsePathStyle:    cfg.S3UsePathStyle,
	})
	if err != nil {
		logger.Warn("s3 adapter init failed; extraction routes will drop tasks",
			"err", err)
	}

	extractor := buildExtractor(cfg, logger)
	thumb := buildThumbnailer(cfg, logger)

	var svc *extractionapp.Service
	if pool != nil && store != nil {
		s, err := extractionapp.NewService(
			libpg.New(pool),
			store,
			extractor,
			cfg.S3BucketRaw,
			cfg.S3BucketArtifacts,
			extractionapp.Options{
				EnabledMimes: cfg.EnabledMimeTypes,
				Logger:       logger,
				Thumbnailer:  thumb,
				Transactor:   readytx.New(pool),
			},
		)
		if err != nil {
			logger.Error("extraction service wiring failed", "err", err)
			os.Exit(1)
		}
		svc = s
	} else {
		logger.Warn("extraction service unavailable — postgres or s3 missing")
	}

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		logger.Error("invalid redis url", "err", err, "url", cfg.RedisURL)
		os.Exit(1)
	}

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: cfg.Concurrency,
		Queues:      cfg.Queues,
		ErrorHandler: asynq.ErrorHandlerFunc(func(_ context.Context, t *asynq.Task, err error) {
			logger.Error("asynq task error",
				"type", t.Type(),
				"err", err,
			)
		}),
	})

	mux := asynq.NewServeMux()
	handlers.Register(mux, handlers.NewExtractHandler(logger, svc))

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

	<-rootCtx.Done()

	logger.Info("shutting down worker")
	srv.Shutdown()
	srv.Stop()
}

// mustOrSkipPool builds a pgx pool and pings it; returns (nil, no-op)
// if Postgres is unreachable so the worker still boots in dev.
func mustOrSkipPool(ctx context.Context, cfg config.Config, logger *slog.Logger) (*pgxpool.Pool, func()) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Warn("postgres unavailable", "err", err)
		return nil, func() {}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Warn("postgres ping failed", "err", err)
		pool.Close()
		return nil, func() {}
	}
	return pool, pool.Close
}

// buildExtractor returns the production MarkItDown adapter when the
// CLI is configured, falling back to the passthrough fake otherwise.
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

// buildThumbnailer returns the pdftoppm adapter when the binary is
// available; otherwise the noop adapter so the worker boots without
// Poppler.
func buildThumbnailer(cfg config.Config, logger *slog.Logger) domain.Thumbnailer {
	if cfg.ThumbnailerBin == "noop" {
		logger.Info("thumbnailer: noop")
		return noopthumbnailer.New()
	}
	return pdftoppm.New(pdftoppm.Config{Bin: cfg.ThumbnailerBin})
}
