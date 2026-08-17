// Package proxy forwards authenticated requests from the SPA to the
// backend API: it validates the session, refreshes the access token if it's
// near expiry, attaches a fresh Authorization header, and relays the
// request. It never derives the destination from anything client-supplied
// — the backend host is fixed at construction time from config, which is
// what makes this a single allowlisted resource server rather than an open
// relay; see the RFC's proxy allowlist requirement.
package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/session"
)

// refreshSkew is how much headroom is required before an access token's
// expiry before it's still treated as usable. Refreshing slightly early
// avoids a request racing a token that expires mid-flight.
const refreshSkew = 30 * time.Second

// allowedMethods is a coarse placeholder until real per-route restrictions
// are known (the RFC recommends restricting methods per endpoint, not just
// per host). Blocks unusual/dangerous methods; everything REST-shaped is
// allowed through.
var allowedMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodOptions: true,
}

// Refresher is the subset of auth.Client's behavior this package depends
// on. Declaring it here (rather than importing internal/auth directly)
// keeps proxy testable with a fake and avoids coupling it to the OIDC
// client's full surface. *auth.Client satisfies this implicitly.
type Refresher interface {
	Refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error)
}

// Handler returns an http.Handler that authenticates, refreshes, and
// forwards requests to cfg.BackendAPIBaseURL, preserving the request path
// unchanged (BACKEND_API_BASE_URL is a bare host with no path component).
// Mount it at /api/* directly — not nested under /bff — since the backend
// already namespaces its own routes under /api; nesting it under
// /bff/api/* would double that prefix on every forwarded request.
func Handler(cfg *config.Config, refresher Refresher, store session.Store) (http.Handler, error) {
	target, err := url.Parse(cfg.BackendAPIBaseURL)
	if err != nil {
		return nil, fmt.Errorf("proxy: parse BACKEND_API_BASE_URL: %w", err)
	}

	rp := &httputil.ReverseProxy{
		// ReverseProxy forwards via RoundTrip, not Client.Do — it never
		// follows redirects on our behalf. A 3xx from the backend is
		// relayed to the caller as-is, satisfying the "no blind
		// redirect-following" requirement without extra code here.
		//
		// Rewrite (not the older Director field, deprecated since Go
		// 1.26): SetURL points the outbound request at the backend
		// (rewriting scheme/host/Host-header; target has no path
		// component, so the incoming path forwards unchanged).
		// SetXForwarded sets X-Forwarded-For/Host/Proto so the backend
		// sees accurate origin info despite the proxy hop.
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy: upstream request failed: %v", err)
			http.Error(w, "upstream request failed", http.StatusBadGateway)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedMethods[r.Method] {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		sessionID, err := session.ReadSessionCookie(r, cfg)
		if err != nil {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}

		data, err := store.Get(r.Context(), sessionID)
		if err != nil {
			http.Error(w, "not authenticated", http.StatusUnauthorized)
			return
		}

		if time.Now().Add(refreshSkew).After(data.ExpiresAt) {
			refreshed, err := refresher.Refresh(r.Context(), data.RefreshToken)
			if err != nil {
				// Per the RFC: once a refresh token is known to be
				// invalid, invalidate the session rather than leaving a
				// dead session around for the client to keep retrying.
				log.Printf("proxy: refresh failed, invalidating session: %v", err)
				_ = store.Delete(r.Context(), sessionID)
				session.ClearSessionCookie(w, cfg)
				http.Error(w, "session expired", http.StatusUnauthorized)
				return
			}

			data.AccessToken = refreshed.AccessToken
			data.ExpiresAt = refreshed.Expiry
			if refreshed.RefreshToken != "" { // Auth0 may rotate it
				data.RefreshToken = refreshed.RefreshToken
			}

			ttl := time.Duration(cfg.SessionTTLSeconds) * time.Second
			if err := store.Save(r.Context(), sessionID, data, ttl); err != nil {
				log.Printf("proxy: save refreshed session: %v", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		// Never forward whatever the caller sent on these headers — only
		// the BFF's own token belongs on the upstream leg, and the
		// session cookie is meaningless (and none of the backend's
		// business) past this point.
		r.Header.Del("Authorization")
		r.Header.Del("Cookie")
		r.Header.Set("Authorization", "Bearer "+data.AccessToken)

		rp.ServeHTTP(w, r)
	}), nil
}
