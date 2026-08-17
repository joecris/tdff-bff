package middleware

import "net/http"

// RequiredHeaderName is the custom header the SPA must send on every
// request to the proxied API. Its value doesn't need to be secret — the
// point (per the RFC's CSRF guidance) is that a cross-site form or <img>
// tag can never set a custom header, so requiring one forces any
// cross-origin fetch/XHR attempt through a CORS preflight, which this
// same-origin deployment doesn't answer. This is defense-in-depth on top
// of SameSite=Strict, not a replacement for it — see the plan's CSRF
// decision for why both layers are in play despite the exclusive domain.
const RequiredHeaderName = "X-Requested-With"

// RequiredHeaderValue is the expected value. Fixed and documented (not
// secret, not randomized per-session) — see RequiredHeaderName.
const RequiredHeaderValue = "XMLHttpRequest"

// RequireCustomHeader rejects any request missing the expected header.
// Intended for state-changing endpoints (the API proxy) — not the
// auth endpoints, which are inherently top-level browser navigations
// (redirects, links) that can never carry custom headers in the first
// place.
func RequireCustomHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(RequiredHeaderName) != RequiredHeaderValue {
			http.Error(w, "missing required header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
