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
	bffmw "github.com/joecris/tdff-bff/internal/middleware"
	"github.com/joecris/tdff-bff/internal/session"
)

// Deps are the dependencies routes need.
type Deps struct {
	Config *config.Config
	Auth   *auth.Client // nil until Phase 2 config/client wiring is present
	Store  session.Store
	Proxy  http.Handler // nil until Phase 4 config wiring is present; see internal/proxy
}

// New builds the top-level router.
func New(deps Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// SecurityHeaders is scoped to this group — the routes the BFF answers
	// itself (health check, auth redirects/errors) — not applied globally.
	// /api/* is deliberately excluded: httputil.ReverseProxy *appends* the
	// backend's own response headers rather than replacing whatever's
	// already on the ResponseWriter, so a blanket r.Use(SecurityHeaders)
	// would leave every proxied response with two, sometimes conflicting,
	// copies of headers like Content-Security-Policy and X-Frame-Options.
	// The backend already sends its own complete set; let it own that for
	// its own responses.
	r.Group(func(r chi.Router) {
		r.Use(bffmw.SecurityHeaders)

		// Unauthenticated infra health check. Deliberately outside any
		// /bff prefix: it's hit directly by Vercel/uptime checks, not
		// proxied same-origin from the SPA.
		r.Get("/healthz", handleHealthz(deps.Config))

		if deps.Auth != nil && deps.Store != nil {
			r.Route("/bff/auth", func(r chi.Router) {
				r.Get("/login", handlers.Login(deps.Auth))
				r.Get("/callback", handlers.Callback(deps.Auth, deps.Store, deps.Config))
				r.Get("/logout", handlers.Logout(deps.Auth, deps.Store, deps.Config))
				r.Get("/session", handlers.SessionInfo(deps.Store, deps.Config))
			})
		}
	})

	if deps.Proxy != nil {
		// Mounted at /api/*, not nested under /bff: the backend already
		// namespaces its own routes under /api, so the path forwards
		// unchanged (see internal/proxy's doc comment). /bff/auth/* stays
		// the only BFF-owned namespace, so there's no collision.
		//
		// CSRF defense-in-depth (bffmw.RequireCustomHeader) is applied
		// here, not on /bff/auth/*: those endpoints are top-level browser
		// navigations (redirects, links) that can never carry a custom
		// header in the first place, so the check would only ever fail.
		r.Handle("/api/*", bffmw.RequireCustomHeader(deps.Proxy))
	}

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
