// Package middleware holds cross-cutting HTTP middleware that isn't
// specific to any one route group: response security headers and the CSRF
// defense-in-depth header check. Request-scoped concerns already covered by
// chi's own middleware (request ID, logging, panic recovery, timeouts) stay
// in internal/router rather than being duplicated here.
package middleware

import "net/http"

// SecurityHeaders sets a conservative set of hardening headers on every
// response. The BFF never serves rendered HTML with its own scripts/styles
// — only JSON, redirects, and plain-text errors — so a strict, blanket CSP
// is safe globally rather than needing a per-route policy.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
		// Sent unconditionally: browsers only honor HSTS on responses
		// received over an actual HTTPS connection, so this is a no-op
		// (and harmless) in local HTTP dev.
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")

		next.ServeHTTP(w, r)
	})
}
