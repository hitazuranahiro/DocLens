// Package clerk implements the Authenticator port against Clerk's JWT.
//
// Clerk signs session tokens with RS256 keys served from the instance's
// JWKS endpoint at `<Issuer>/.well-known/jwks.json`. We verify
// signatures, expiry, issuer, and audience here.
//
// The JWKS is fetched lazily and refreshed in the background by the
// keyfunc library; rotated keys are picked up without a process
// restart.
package clerk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/tomeku/doclens/services/shared/auth"
)

// Config carries the values needed to verify Clerk JWTs.
type Config struct {
	// Issuer is the Clerk Frontend API URL, e.g.
	// "https://your-app.clerk.accounts.dev". The JWKS endpoint
	// is derived as `<Issuer>/.well-known/jwks.json`.
	Issuer string
	// Audience, when non-empty, is checked against the `aud` claim.
	// Clerk session tokens carry the Frontend API URL as `azp`, not
	// `aud`, so most setups leave this empty and rely on `iss` +
	// signature.
	Audience string
	// JWKSRefreshInterval controls how often the cached JWKS is
	// refreshed in the background. Defaults to 10m.
	JWKSRefreshInterval time.Duration
}

// Authenticator verifies Clerk-issued JWTs.
type Authenticator struct {
	cfg     Config
	keyFunc jwt.Keyfunc
}

// New returns a Clerk Authenticator that fetches the JWKS lazily.
//
// It returns an error when the JWKS endpoint can't be reached at
// startup — fail-fast so a misconfigured deploy is loud, rather than
// silently 401'ing every request.
func New(cfg Config) (*Authenticator, error) {
	if cfg.Issuer == "" {
		return nil, errors.New("clerk: Issuer is required")
	}
	cfg.Issuer = strings.TrimRight(cfg.Issuer, "/")
	if cfg.JWKSRefreshInterval <= 0 {
		cfg.JWKSRefreshInterval = 10 * time.Minute
	}

	jwksURL := cfg.Issuer + "/.well-known/jwks.json"
	k, err := keyfunc.NewDefaultCtx(context.Background(), []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("clerk: load JWKS from %s: %w", jwksURL, err)
	}
	return &Authenticator{cfg: cfg, keyFunc: k.Keyfunc}, nil
}

// Verify parses, validates, and extracts identity from a Clerk JWT.
//
// Validation contract:
//
//   - Signature against the cached JWKS (RS256).
//   - `exp` not past (jwt/v5 enforces this by default).
//   - `nbf` not future (also default).
//   - `iss` matches Config.Issuer exactly.
//   - When Config.Audience is non-empty, `aud` must contain it.
//
// Returns auth.ErrInvalidToken on any signature/claim failure (the
// HTTP layer maps this to 401 with a generic message — we don't
// surface JWT internals to the client).
func (a *Authenticator) Verify(ctx context.Context, token string) (auth.Identity, error) {
	if token == "" {
		return auth.Identity{}, auth.ErrMissingToken
	}

	parsed, err := jwt.ParseWithClaims(token, &clerkClaims{}, a.keyFunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(a.cfg.Issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return auth.Identity{}, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
	}
	claims, ok := parsed.Claims.(*clerkClaims)
	if !ok || !parsed.Valid {
		return auth.Identity{}, auth.ErrInvalidToken
	}
	if a.cfg.Audience != "" && !claims.hasAudience(a.cfg.Audience) {
		return auth.Identity{}, fmt.Errorf("%w: audience mismatch", auth.ErrInvalidToken)
	}
	if claims.Subject == "" {
		return auth.Identity{}, fmt.Errorf("%w: missing sub", auth.ErrInvalidToken)
	}

	return auth.Identity{
		UserID:      claims.Subject,
		Email:       claims.Email,
		DisplayName: claims.displayName(),
	}, nil
}

// clerkClaims maps the subset of Clerk's session claims we use.
//
// Clerk's session JWT carries:
//   - sub:   user_xxxxx
//   - email: primary email address (when a session-claim template is set)
//   - email_addresses, first_name, last_name (templated)
//   - iss, exp, nbf, iat, jti
//
// Out of the box, only `sub` is guaranteed. Email and display-name
// are optional; we fall back to sub when missing.
type clerkClaims struct {
	jwt.RegisteredClaims
	Email     string `json:"email,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

func (c *clerkClaims) displayName() string {
	switch {
	case c.FirstName != "" && c.LastName != "":
		return c.FirstName + " " + c.LastName
	case c.FirstName != "":
		return c.FirstName
	case c.LastName != "":
		return c.LastName
	case c.Email != "":
		return c.Email
	default:
		return c.Subject
	}
}

func (c *clerkClaims) hasAudience(want string) bool {
	for _, a := range c.Audience {
		if a == want {
			return true
		}
	}
	return false
}
