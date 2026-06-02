// Package transport — CORS middleware for browser-direct API calls.
//
// Most of DocLens's read-side traffic is proxied through Next route
// handlers (markdown, thumbnail, raw, stream, delete) which inject
// the Clerk Bearer token server-side. The upload flow can't use that
// pattern: the browser needs to call /v1/uploads, then PUT to S3,
// then call /v1/documents/{id}/finalize, all stitched together in
// one user gesture. That makes /v1/uploads a true browser-direct
// endpoint, which means CORS.
//
// The contract:
//
//   - Allowed origins are configured by the caller (e.g. "http://localhost:3000"
//     in dev, "https://app.example.com" in prod).
//   - When the request's Origin matches an allowed entry, we mirror it
//     back as Access-Control-Allow-Origin and add the credential +
//     methods + headers Clerk-token uploads need.
//   - Preflight (OPTIONS) short-circuits with 204 before auth runs.
//   - Wildcard "*" is supported for development; not recommended in
//     production with credentials (the browser blocks the combination).
package transport

import (
	"net/http"
	"strings"
)

// CORSConfig bundles allowed origins and a few small knobs.
type CORSConfig struct {
	// AllowedOrigins is matched exactly against the Origin header.
	// "*" matches everything (dev-only).
	AllowedOrigins []string
	// MaxAgeSeconds is the preflight cache TTL. 600 is a sensible
	// default; browsers cap most values to 7200 anyway.
	MaxAgeSeconds int
}

// CORS returns a middleware enforcing AllowedOrigins. When the list
// is empty the middleware is a passthrough (no CORS headers added),
// so production deployments that pin origins via reverse proxy don't
// have to disable this layer.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	wildcard := false
	for _, o := range cfg.AllowedOrigins {
		o = strings.TrimSpace(o)
		if o == "*" {
			wildcard = true
			continue
		}
		if o != "" {
			allowed[o] = struct{}{}
		}
	}
	maxAge := cfg.MaxAgeSeconds
	if maxAge <= 0 {
		maxAge = 600
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && (wildcard || originAllowed(allowed, origin)) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// Required to vary on Origin so caches don't serve a
				// response with the wrong allow-origin to a different caller.
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods",
					"GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers",
					"Authorization, Content-Type, X-Requested-With")
				w.Header().Set("Access-Control-Expose-Headers", "Content-Length")
				w.Header().Set("Access-Control-Max-Age", itoa(maxAge))
			}

			// Short-circuit preflights so they never hit the auth gate.
			// CORS preflights deliberately omit credentials, so the
			// browser would otherwise see an unhelpful 401.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func originAllowed(set map[string]struct{}, origin string) bool {
	_, ok := set[origin]
	return ok
}

// itoa avoids strconv import; max-age never exceeds int range.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
