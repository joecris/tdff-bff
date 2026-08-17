// Package router assembles the BFF's chi router: middleware chain plus
// route mounts. Route groups (auth, proxy) are added by later phases via
// their own Mount functions, keeping this file a thin composition point
// rather than a growing pile of handler logic.
package router

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/joecris/tdff-bff/internal/auth"
	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/handlers"
	"github.com/joecris/tdff-bff/internal/session"
)

// Deps are the dependencies routes need. Phase 4 will add a proxy target
// here; Phase 3 swaps Store's concrete type (memory -> Redis) without any
// change to this struct's shape.
type Deps struct {
	Config *config.Config
	Auth   *auth.Client // nil until Phase 2 config/client wiring is present
	Store  session.Store
}

// New builds the top-level router.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Unauthenticated infra health check. Deliberately outside any /bff
	// prefix: it's hit directly by Vercel/uptime checks, not proxied
	// same-origin from the SPA.
	r.Get("/healthz", handleHealthz(deps.Config))

	if deps.Auth != nil && deps.Store != nil {
		r.Route("/bff/auth", func(r chi.Router) {
			r.Get("/login", handlers.Login(deps.Auth))
			r.Get("/callback", handlers.Callback(deps.Auth, deps.Store, deps.Config))
			r.Get("/logout", handlers.Logout(deps.Auth, deps.Store, deps.Config))
			r.Get("/session", handlers.SessionInfo(deps.Store, deps.Config))
		})
	}

	// TODO(phase 4): mount proxy.Routes(r, ...) under /bff/api

	return r
}

type healthzResponse struct {
	Status string `json:"status"`
	Env    string `json:"env"`
}

func handleHealthz(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(healthzResponse{
			Status: "ok",
			Env:    cfg.AppEnv,
		})
	}
}
