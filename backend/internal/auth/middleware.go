package auth

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/httpx"
)

type ctxKey struct{}

// ClaimsFrom returns the logged-in staff claims that Middleware put on the request.
func ClaimsFrom(ctx context.Context) (Claims, bool) {
	c, ok := ctx.Value(ctxKey{}).(Claims)
	return c, ok
}

// Middleware blocks requests without a valid "Authorization: Bearer <token>".
func Middleware(tm *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				unauthorized(w, "missing token")
				return
			}
			claims, err := tm.Parse(token)
			if err != nil {
				unauthorized(w, err.Error())
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
		})
	}
}

// RequireRole only lets staff with one of the given roles through.
// Use it after Middleware.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFrom(r.Context())
			if !ok {
				unauthorized(w, "missing token")
				return
			}
			if !slices.Contains(roles, claims.Role) {
				httpx.WriteError(w, http.StatusForbidden, "you do not have access to this")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	httpx.WriteError(w, http.StatusUnauthorized, msg)
}
