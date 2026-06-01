// Package local implements a development-only Authenticator that accepts
// opaque tokens of the form "dev:<userId>:<email>".
//
// This adapter exists for local development and tests. It MUST NOT be
// enabled in production (the API config validator rejects AUTH_PROVIDER=local
// when GO_ENV=production).
package local

import (
	"context"
	"strings"

	"github.com/tomeku/doclens/services/shared/auth"
)

// Authenticator is the dev-mode adapter.
type Authenticator struct{}

// New returns a ready-to-use local Authenticator.
func New() *Authenticator { return &Authenticator{} }

// Verify parses tokens shaped "dev:<userId>:<email>".
//
// Any other shape returns auth.ErrInvalidToken.
func (a *Authenticator) Verify(_ context.Context, token string) (auth.Identity, error) {
	if token == "" {
		return auth.Identity{}, auth.ErrMissingToken
	}
	parts := strings.SplitN(token, ":", 3)
	if len(parts) != 3 || parts[0] != "dev" || parts[1] == "" || parts[2] == "" {
		return auth.Identity{}, auth.ErrInvalidToken
	}
	return auth.Identity{
		UserID:      parts[1],
		Email:       parts[2],
		DisplayName: parts[1],
	}, nil
}
