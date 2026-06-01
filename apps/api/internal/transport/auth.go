package transport

import (
	"context"
	"net/http"
	"strings"

	"github.com/tomeku/doclens/services/shared/auth"
)

type identityCtxKey struct{}

// IdentityFrom returns the authenticated identity from the request context.
//
// ok is false when the request was not gated by AuthMiddleware. Handlers
// behind the middleware can rely on ok == true.
func IdentityFrom(ctx context.Context) (auth.Identity, bool) {
	id, ok := ctx.Value(identityCtxKey{}).(auth.Identity)
	return id, ok
}

// withIdentity attaches the identity to the request context.
func withIdentity(ctx context.Context, id auth.Identity) context.Context {
	return context.WithValue(ctx, identityCtxKey{}, id)
}

// AuthMiddleware verifies the bearer token using the supplied Authenticator
// and attaches the resulting Identity to the request context. On failure
// it writes an RFC 7807 problem document with status 401.
func AuthMiddleware(a auth.Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerFrom(r.Header.Get("Authorization"))
			if token == "" {
				WriteProblem(w, http.StatusUnauthorized, "Unauthorized",
					"missing bearer token")
				return
			}
			id, err := a.Verify(r.Context(), token)
			if err != nil {
				WriteProblem(w, http.StatusUnauthorized, "Unauthorized",
					"invalid bearer token")
				return
			}
			next.ServeHTTP(w, r.WithContext(withIdentity(r.Context(), id)))
		})
	}
}

func bearerFrom(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
