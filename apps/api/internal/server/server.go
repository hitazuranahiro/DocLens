// Package server wires the chi router with middleware and routes.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	gen "github.com/tomeku/doclens/apps/api/internal/generated/api"
	"github.com/tomeku/doclens/apps/api/internal/handlers"
	"github.com/tomeku/doclens/apps/api/internal/transport"
	"github.com/tomeku/doclens/services/shared/auth"
)

// New returns a fully-wired http.Handler for the API.
//
// The auth middleware is applied to every route except the public ones
// listed in publicPaths.
func New(a auth.Authenticator) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(authGate(a))

	srv := handlers.New()
	gen.HandlerFromMux(srv, r)
	return r
}

// publicPaths are not gated by auth.
var publicPaths = map[string]bool{
	"/v1/health": true,
}

// authGate skips publicPaths and requires bearer auth on everything else.
func authGate(a auth.Authenticator) func(http.Handler) http.Handler {
	mw := transport.AuthMiddleware(a)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if publicPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}
			mw(next).ServeHTTP(w, r)
		})
	}
}
