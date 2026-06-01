// Command api is the DocLens HTTP gateway.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tomeku/doclens/apps/api/internal/config"
	"github.com/tomeku/doclens/apps/api/internal/server"
	"github.com/tomeku/doclens/services/shared/auth"
	"github.com/tomeku/doclens/services/shared/auth/clerk"
	"github.com/tomeku/doclens/services/shared/auth/local"
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

	authn := buildAuthenticator(cfg, logger)
	handler := server.New(authn)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
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
