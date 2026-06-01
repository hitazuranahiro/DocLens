// Package auth defines the Authenticator port that the API uses to verify
// bearer tokens. Adapters live in sibling packages (clerk, local).
//
// Per ADR 0008, the API does not depend on any specific identity provider.
// All token verification happens through the Authenticator interface.
package auth

import (
	"context"
	"errors"
)

// ErrInvalidToken is returned when a token cannot be verified.
var ErrInvalidToken = errors.New("auth: invalid token")

// ErrMissingToken is returned when no Authorization header is supplied.
var ErrMissingToken = errors.New("auth: missing token")

// Identity represents the authenticated user.
//
// Claims carries provider-specific data; the API treats it as opaque.
type Identity struct {
	UserID      string
	Email       string
	DisplayName string
	Claims      map[string]any
}

// Authenticator verifies bearer tokens and returns the authenticated Identity.
//
// Implementations MUST return ErrInvalidToken for tokens that fail any
// verification step (signature, expiry, audience). Implementations SHOULD NOT
// leak provider-specific error details into wrapped errors that travel to
// the client.
type Authenticator interface {
	Verify(ctx context.Context, token string) (Identity, error)
}
