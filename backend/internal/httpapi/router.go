package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/auth"
	"github.com/ctrl-md/patient-intake-and-triage/backend/internal/httpx"
)

// Pinger is anything that can check if the database is alive.
// The pgx pool satisfies it, and tests can use a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps is everything the router needs. Auth routes are only added when
// Auth is set, so the health check can be tested on its own.
type Deps struct {
	DB     Pinger
	Auth   *auth.Handler
	Tokens *auth.TokenManager
}

func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler(d.DB))

	if d.Auth != nil {
		requireLogin := auth.Middleware(d.Tokens)

		mux.HandleFunc("POST /auth/register", d.Auth.Register)
		mux.HandleFunc("POST /auth/login", d.Auth.Login)
		mux.Handle("GET /auth/me", requireLogin(http.HandlerFunc(d.Auth.Me)))
	}
	return mux
}

func healthHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status":   "degraded",
				"database": "down",
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"database": "up",
		})
	}
}
