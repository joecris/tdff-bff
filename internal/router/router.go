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

	"github.com/joecris/tdff-bff/internal/config"
)

// New builds the top-level router. Deps that later phases' route groups
// need (session store, Auth0 client, proxy) get threaded through here once
// those packages exist — Phase 1 only has the health check.
func New(cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Unauthenticated infra health check. Deliberately outside any /bff
	// prefix: it's hit directly by Vercel/uptime checks, not proxied
	// same-origin from the SPA.
	r.Get("/healthz", handleHealthz(cfg))

	// TODO(phase 2): mount auth.Routes(r, ...) under /bff/auth
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
