// Package clerk implements the Authenticator port against Clerk's JWT.
//
// Clerk signs session tokens with RS256/ES256 keys served from the
// instance's JWKS endpoint. We verify signatures, expiry, issuer, and
// audience here. Caching the JWKS is the responsibility of the
// constructor; we use a short refresh interval (10m) to pick up rotated
// keys without restarting the process.
//
// The full implementation lands together with the JWKS client in M1.5.
// Until then, calling Verify returns auth.ErrInvalidToken so that the
// system fails closed.
package clerk

import (
	"context"

	"github.com/tomeku/doclens/services/shared/auth"
)

// Config carries the values needed to verify Clerk JWTs.
type Config struct {
	// Issuer is the Clerk Frontend API URL, e.g. "https://your-app.clerk.accounts.dev".
	Issuer string
	// Audience, when non-empty, is checked against the `aud` claim.
	Audience string
}

// Authenticator verifies Clerk-issued JWTs.
type Authenticator struct {
	cfg Config
}

// New returns a Clerk Authenticator. The JWKS-backed verifier is wired in M1.5.
func New(cfg Config) *Authenticator { return &Authenticator{cfg: cfg} }

// Verify is a placeholder that fails closed. Replaced in M1.5 with
// signature verification against the Clerk JWKS.
func (a *Authenticator) Verify(_ context.Context, _ string) (auth.Identity, error) {
	return auth.Identity{}, auth.ErrInvalidToken
}
