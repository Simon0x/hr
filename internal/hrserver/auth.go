package hrserver

import (
	"context"
	"net/http"
	"strings"

	"github.com/Simon0x/hr/internal/pgstore"
)

type identityContextKey struct{}

// identityFromContext returns the identity withAuth resolved for this
// request. Only meaningful inside a handler reached through withAuth.
func identityFromContext(ctx context.Context) (pgstore.Identity, bool) {
	id, ok := ctx.Value(identityContextKey{}).(pgstore.Identity)
	return id, ok
}

// bearerToken reads the token from the Authorization header, falling back
// to a ?token= query parameter - EventSource cannot set custom headers, so
// the two SSE stream endpoints have no other way to authenticate.
func bearerToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		if tok, ok := strings.CutPrefix(auth, "Bearer "); ok {
			return tok
		}
	}
	return r.URL.Query().Get("token")
}

// withAuth resolves the request's bearer token to a real identity and
// rejects the request if it has none. Every /v1/ route is wrapped in this -
// see Handler().
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok, err := pgstore.IdentityByToken(r.Context(), s.Pool, bearerToken(r))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			msg := "missing or invalid bearer token"
			if any, herr := pgstore.HasAnyIdentity(r.Context(), s.Pool); herr == nil && !any {
				msg = "no identities exist yet - run `hr identity create --name <you> --departments all` and use the printed token"
			}
			http.Error(w, msg, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), identityContextKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
