// Package handlers implements the API ServerInterface generated from
// openapi.yaml. Each method is small and delegates business work to use
// cases (added in later milestones).
package handlers

import (
	"net/http"
	"time"

	gen "github.com/tomeku/doclens/apps/api/internal/generated/api"
	"github.com/tomeku/doclens/apps/api/internal/transport"
	"github.com/tomeku/doclens/services/shared/version"
)

// Server is the concrete implementation of gen.ServerInterface.
type Server struct {
	startedAt time.Time
}

// New returns a Server ready to be wired into the chi router.
func New() *Server {
	return &Server{startedAt: time.Now()}
}

// GetHealth implements GET /v1/health.
func (s *Server) GetHealth(w http.ResponseWriter, _ *http.Request) {
	uptime := int64(time.Since(s.startedAt).Seconds())
	writeJSON(w, http.StatusOK, gen.Health{
		Status:        gen.Ok,
		Version:       version.Version,
		Commit:        ptr(version.Commit),
		UptimeSeconds: uptime,
	})
}

// GetMe implements GET /v1/me. The auth middleware has already verified
// the bearer token; we only render the Identity.
func (s *Server) GetMe(w http.ResponseWriter, r *http.Request) {
	id, ok := transport.IdentityFrom(r.Context())
	if !ok {
		// Unreachable in production: the route is mounted behind AuthMiddleware.
		transport.WriteProblem(w, http.StatusUnauthorized,
			"Unauthorized", "no identity in context")
		return
	}
	out := gen.Identity{
		UserId: id.UserID,
		Email:  openapiEmail(id.Email),
	}
	if id.DisplayName != "" {
		dn := id.DisplayName
		out.DisplayName = &dn
	}
	writeJSON(w, http.StatusOK, out)
}

// --- helpers --------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := newJSONEncoder(w)
	_ = enc.Encode(body)
}

func ptr[T any](v T) *T { return &v }
