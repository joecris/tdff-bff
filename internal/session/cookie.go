package session

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/joecris/tdff-bff/internal/config"
)

// NewID generates a cryptographically random, URL-safe session ID. This is
// the only value the browser ever sees — it carries no information of its
// own and is meaningless without a Store lookup.
func NewID() string {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read only fails if the OS CSPRNG is unavailable,
		// which means the process environment is broken beyond recovery.
		panic("session: crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// SetSessionCookie issues the long-lived session cookie. SameSite=Strict is
// safe here (unlike the auth transaction cookie) because by the time this
// is set, the browser is done round-tripping through Auth0 — every request
// that will carry this cookie is same-site from here on.
func SetSessionCookie(w http.ResponseWriter, id string, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   cfg.SessionTTLSeconds,
	})
}

// ReadSessionCookie extracts the session ID from the request, if present.
func ReadSessionCookie(r *http.Request, cfg *config.Config) (string, error) {
	c, err := r.Cookie(cfg.SessionCookieName)
	if err != nil || c.Value == "" {
		return "", ErrNoSession
	}
	return c.Value, nil
}

// ClearSessionCookie expires the session cookie on the browser. Attributes
// must match SetSessionCookie's for the browser to actually overwrite it.
func ClearSessionCookie(w http.ResponseWriter, cfg *config.Config) {
	http.SetCookie(w, &http.Cookie{
		Name:     cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
}
