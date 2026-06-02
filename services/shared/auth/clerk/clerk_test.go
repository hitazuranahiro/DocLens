// Tests for the Clerk authenticator.
//
// We spin up a tiny local HTTPS-style server that serves a JWKS
// derived from a test RSA keypair, then sign tokens with that
// keypair and assert Verify accepts/rejects the expected cases.
//
// This keeps the test hermetic: no network, no real Clerk instance.

package clerk

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/tomeku/doclens/services/shared/auth"
)

func TestVerify_Accepts_ValidToken(t *testing.T) {
	signer, srv := newJWKSServer(t)
	t.Cleanup(srv.Close)

	a, err := New(Config{Issuer: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tok := signer.sign(t, jwt.MapClaims{
		"sub":   "user_test_42",
		"email": "alice@example.com",
		"iss":   srv.URL,
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
		"iat":   time.Now().Unix(),
	})

	id, err := a.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id.UserID != "user_test_42" {
		t.Fatalf("UserID = %q, want user_test_42", id.UserID)
	}
	if id.Email != "alice@example.com" {
		t.Fatalf("Email = %q, want alice@example.com", id.Email)
	}
}

func TestVerify_Rejects_ExpiredToken(t *testing.T) {
	signer, srv := newJWKSServer(t)
	t.Cleanup(srv.Close)

	a, _ := New(Config{Issuer: srv.URL})
	tok := signer.sign(t, jwt.MapClaims{
		"sub": "user_test_42",
		"iss": srv.URL,
		"exp": time.Now().Add(-1 * time.Minute).Unix(),
		"iat": time.Now().Add(-2 * time.Minute).Unix(),
	})

	if _, err := a.Verify(context.Background(), tok); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken on expired, got %v", err)
	}
}

func TestVerify_Rejects_WrongIssuer(t *testing.T) {
	signer, srv := newJWKSServer(t)
	t.Cleanup(srv.Close)

	a, _ := New(Config{Issuer: srv.URL})
	tok := signer.sign(t, jwt.MapClaims{
		"sub": "user_test_42",
		"iss": "https://attacker.example.com",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	if _, err := a.Verify(context.Background(), tok); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken on wrong issuer, got %v", err)
	}
}

func TestVerify_Rejects_TamperedSignature(t *testing.T) {
	signer, srv := newJWKSServer(t)
	t.Cleanup(srv.Close)

	a, _ := New(Config{Issuer: srv.URL})
	tok := signer.sign(t, jwt.MapClaims{
		"sub": "user_test_42",
		"iss": srv.URL,
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})
	// Flip a byte in the signature segment.
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}
	tampered := parts[0] + "." + parts[1] + "." + flipFirstChar(parts[2])

	if _, err := a.Verify(context.Background(), tampered); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken on tampered sig, got %v", err)
	}
}

func TestVerify_Rejects_MissingSub(t *testing.T) {
	signer, srv := newJWKSServer(t)
	t.Cleanup(srv.Close)

	a, _ := New(Config{Issuer: srv.URL})
	tok := signer.sign(t, jwt.MapClaims{
		"iss": srv.URL,
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})

	if _, err := a.Verify(context.Background(), tok); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken on missing sub, got %v", err)
	}
}

func TestVerify_RejectsHS256_AlgConfusion(t *testing.T) {
	_, srv := newJWKSServer(t)
	t.Cleanup(srv.Close)

	a, _ := New(Config{Issuer: srv.URL})

	// A token signed with HS256 using the public key bytes — the
	// classic alg-confusion attack. ParseWithClaims is configured
	// to only accept RS256, so this must fail.
	hsTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "user_evil",
		"iss": srv.URL,
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})
	signed, err := hsTok.SignedString([]byte("ignored"))
	if err != nil {
		t.Fatalf("sign HS256: %v", err)
	}
	if _, err := a.Verify(context.Background(), signed); !errors.Is(err, auth.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken on HS256, got %v", err)
	}
}

func TestVerify_Empty_MissingTokenError(t *testing.T) {
	_, srv := newJWKSServer(t)
	t.Cleanup(srv.Close)

	a, _ := New(Config{Issuer: srv.URL})
	if _, err := a.Verify(context.Background(), ""); !errors.Is(err, auth.ErrMissingToken) {
		t.Fatalf("expected ErrMissingToken on empty, got %v", err)
	}
}

// --- test helpers ---------------------------------------------------------

type testSigner struct {
	priv *rsa.PrivateKey
	kid  string
}

func (s *testSigner) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = s.kid
	signed, err := tok.SignedString(s.priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

// newJWKSServer returns an httptest server that serves a JWKS
// containing the public half of a freshly-minted RSA keypair, and
// a signer wrapping the private half.
func newJWKSServer(t *testing.T) (*testSigner, *httptest.Server) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa gen: %v", err)
	}
	kid := "test-key-1"
	signer := &testSigner{priv: priv, kid: kid}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, jwksJSON(priv, kid))
	})
	srv := httptest.NewServer(mux)
	return signer, srv
}

// jwksJSON renders a minimal RFC 7517 JWKS for one RSA public key.
func jwksJSON(priv *rsa.PrivateKey, kid string) string {
	pub := priv.PublicKey
	doc := map[string]any{
		"keys": []map[string]string{
			{
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"kid": kid,
				"n":   base64URLBigInt(pub.N),
				"e":   base64URLInt(pub.E),
			},
		},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

func base64URLBigInt(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

func base64URLInt(e int) string {
	bi := big.NewInt(int64(e))
	return base64.RawURLEncoding.EncodeToString(bi.Bytes())
}

func flipFirstChar(s string) string {
	if s == "" {
		return "X"
	}
	first := s[0]
	if first == 'A' {
		first = 'B'
	} else {
		first = 'A'
	}
	return string(first) + s[1:]
}
