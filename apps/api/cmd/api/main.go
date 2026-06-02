// Command api is the DocLens HTTP gateway.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"

	"github.com/tomeku/doclens/apps/api/internal/config"
	"github.com/tomeku/doclens/apps/api/internal/handlers"
	"github.com/tomeku/doclens/apps/api/internal/librarytx"
	"github.com/tomeku/doclens/apps/api/internal/pubsub"
	"github.com/tomeku/doclens/apps/api/internal/server"
	"github.com/tomeku/doclens/apps/api/internal/sweeper"
	ingapp "github.com/tomeku/doclens/services/ingestion/app"
	ingpg "github.com/tomeku/doclens/services/ingestion/adapters/postgres"
	libapp "github.com/tomeku/doclens/services/library/app"
	libpg "github.com/tomeku/doclens/services/library/adapters/postgres"
	searchpg "github.com/tomeku/doclens/services/search/adapters/postgres"
	searchapp "github.com/tomeku/doclens/services/search/app"
	"github.com/tomeku/doclens/services/shared/auth"
	"github.com/tomeku/doclens/services/shared/auth/clerk"
	"github.com/tomeku/doclens/services/shared/auth/local"
	"github.com/tomeku/doclens/services/shared/jobs"
	jobsasynq "github.com/tomeku/doclens/services/shared/jobs/asynq"
	jobsinmem "github.com/tomeku/doclens/services/shared/jobs/inmem"
	"github.com/tomeku/doclens/services/shared/storage"
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

	deps, cleanup, err := buildDeps(rootCtx, cfg, logger)
	if err != nil {
		logger.Error("dependency wiring failed", "err", err)
		os.Exit(1)
	}
	defer cleanup()

	if deps.Handlers.Uploads != nil {
		go sweeper.Run(rootCtx, deps.Handlers.Uploads, cfg.SweepInterval, logger)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.New(deps),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api listening",
			"addr", cfg.HTTPAddr,
			"env", cfg.Env,
			"auth_provider", cfg.AuthProvider)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()

	<-rootCtx.Done()

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
}

// buildDeps wires every collaborator. It returns a cleanup that closes
// any pools/clients we opened.
//
// If Postgres or S3 is unavailable at startup the API still serves
// /v1/health and /v1/me; upload routes return 503 from the handler.
// This makes the bootstrap experience friendlier (`make dev` doesn't
// race the compose stack).
func buildDeps(ctx context.Context, cfg config.Config, logger *slog.Logger) (server.Deps, func(), error) {
	cleanup := func() {}

	authn := buildAuthenticator(cfg, logger)

	// Object storage. We don't fail startup if this fails; the upload
	// routes will report 503 until the dependency comes up.
	var store storage.ObjectStore
	if s3a, err := storages3.New(ctx, storages3.Config{
		Endpoint:        cfg.S3Endpoint,
		Region:          cfg.S3Region,
		AccessKeyID:     cfg.S3AccessKeyID,
		SecretAccessKey: cfg.S3SecretAccessKey,
		UsePathStyle:    cfg.S3UsePathStyle,
	}); err != nil {
		logger.Warn("s3 adapter unavailable", "err", err)
	} else {
		store = s3a
	}

	// Postgres. Same forgiving startup behavior — we want the API to
	// boot in offline dev too.
	var pool *pgxpool.Pool
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Warn("postgres unavailable", "err", err)
		pool = nil
	} else {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := pool.Ping(pingCtx); err != nil {
			logger.Warn("postgres ping failed", "err", err)
			pool.Close()
			pool = nil
		}
		cancel()
	}
	if pool != nil {
		cleanup = func() { pool.Close() }
	}

	deps := server.Deps{Auth: authn}
	if pool != nil && store != nil {
		bus := buildJobBus(cfg, logger)
		uploads := ingapp.NewServiceMust(
			ingpg.New(pool),
			libpg.New(pool),
			store,
			cfg.S3BucketRaw,
			ingapp.Options{
				PresignTTL:  cfg.UploadPresignTTL,
				EnabledMime: cfg.EnabledMimeTypes,
				Bus:         bus,
				Logger:      logger,
			},
		)
		library, err := libapp.NewService(libpg.New(pool), store, cfg.S3BucketRaw, cfg.S3BucketArtifacts)
		if err != nil {
			return server.Deps{}, cleanup, fmt.Errorf("library service: %w", err)
		}

		searchSvc, err := searchapp.NewService(searchpg.New(pool))
		if err != nil {
			return server.Deps{}, cleanup, fmt.Errorf("search service: %w", err)
		}

		// Wire delete: tx around library + search index, async S3
		// cleanup. The search Repo doubles as the IndexEraser via
		// the librarytx adapter.
		library.SetDeleteDeps(
			librarytx.New(pool),
			&searchEraser{repo: searchpg.New(pool)},
			store,
			logger,
		)

		// Live-status SSE: a dedicated pgx.Conn handles LISTEN; the
		// hub fans out to in-process SSE subscribers. The listener
		// reconnects on its own; we don't fail startup on its first
		// hiccup.
		hub := pubsub.NewHub(0)
		listener := pubsub.NewListener(
			pubsub.PgxConnector(cfg.DatabaseURL),
			hub,
			pubsub.Options{Logger: logger},
		)
		go func() {
			if err := listener.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Error("pubsub: listener stopped", "err", err)
			}
		}()

		deps.Handlers = handlers.Deps{
			Uploads: uploads,
			Library: library,
			Hub:     hub,
			Search:  searchSvc,
		}
	} else {
		logger.Warn("upload + library routes disabled — postgres or s3 unavailable")
	}

	return deps, cleanup, nil
}

// buildJobBus picks the asynq adapter when Redis is reachable; falls
// back to the in-memory bus otherwise so dev contributors can test
// the API without Redis. Production callers must have Redis up.
func buildJobBus(cfg config.Config, logger *slog.Logger) jobs.JobBus {
	bus, err := jobsasynq.New(cfg.RedisURL)
	if err != nil {
		logger.Warn("asynq bus unavailable; jobs will be recorded in-process",
			"err", err,
			"redis_url", cfg.RedisURL,
		)
		return jobsinmem.NewBus()
	}
	logger.Info("asynq bus connected", "redis_url", cfg.RedisURL)
	return bus
}

func buildAuthenticator(cfg config.Config, logger *slog.Logger) auth.Authenticator {
	switch cfg.AuthProvider {
	case config.ProviderLocal:
		logger.Warn("auth provider: local (development only)")
		return local.New()
	case config.ProviderClerk:
		return clerk.New(clerk.Config{
			Issuer:   cfg.ClerkIssuer,
			Audience: cfg.ClerkAud,
		})
	default:
		logger.Error("unsupported auth provider", "provider", cfg.AuthProvider)
		os.Exit(1)
		return nil
	}
}


// searchEraser bridges the search postgres Repo to the library's
// IndexEraser port for the non-transactional path. The transactional
// path uses librarytx's bound adapter; this fallback only fires when
// the transactor encounters an error before the callback runs.
type searchEraser struct {
	repo *searchpg.Repo
}

func (s *searchEraser) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
