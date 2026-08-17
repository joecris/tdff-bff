package session

import (
	"net/url"
	"strings"
)

// SanitizeReturnTo validates a caller-supplied post-login redirect target
// and returns it unchanged if safe, or "" if not. This exists to let
// /bff/auth/login accept a ?returnTo=/some/path query param (so the
// callback can send the browser back to whatever SPA route it started
// from, instead of always a fixed home page) without opening a classic
// open-redirect hole: raw is untrusted input that flows straight into an
// http.Redirect target, so anything that could resolve to a different
// origin — an absolute URL, a protocol-relative "//host/path", a
// backslash a browser might normalize like a forward slash — must be
// rejected. Only a same-origin-relative path (leading single "/") is
// accepted.
func SanitizeReturnTo(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "\\") {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.IsAbs() || u.Host != "" || u.Opaque != "" {
		return ""
	}
	if !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return ""
	}

	return raw
}
