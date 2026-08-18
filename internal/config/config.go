// Package config loads and validates the BFF's runtime configuration from
// environment variables. Values are read once at startup; nothing here
// re-reads the environment later.
package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds every environment-derived setting the BFF needs. Fields are
// grouped by the feature that owns them; groups introduced in later phases
// (Auth0, Redis, backend API) are populated now so the shape is stable, but
// are only required (see Validate) once the feature that uses them lands.
type Config struct {
	// Core / Phase 1
	AppEnv   string // "nonprod" | "prod" | "development"
	Port     string
	LogLevel string

	// Auth0 — required starting Phase 2 (auth flow)
	Auth0Domain            string
	Auth0ClientID          string
	Auth0ClientSecret      string
	Auth0Audience          string
	Auth0CallbackURL       string
	Auth0LogoutRedirectURL string

	// PostLoginRedirectURL is where the browser is sent after a successful
	// callback. Defaults to Auth0LogoutRedirectURL (the SPA base URL) when
	// unset, so it doesn't need to be a required var — set it explicitly
	// once login and logout should land on different routes.
	PostLoginRedirectURL string

	// Session / Redis — required starting Phase 3 (session + Redis)
	RedisURL          string
	SessionCookieName string
	SessionTTLSeconds int

	// Backend API proxy — required starting Phase 4 (proxy)
	BackendAPIBaseURL string
}

// Load reads Config from the process environment, applying defaults for
// local development. It does not validate feature-specific required fields;
// call the relevant Require* method once that feature's handlers are wired
// in, so an incomplete .env doesn't block phases that don't need those vars.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:   getEnv("APP_ENV", "development"),
		Port:     getEnv("PORT", "8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		Auth0Domain:            os.Getenv("AUTH0_DOMAIN"),
		Auth0ClientID:          os.Getenv("AUTH0_CLIENT_ID"),
		Auth0ClientSecret:      os.Getenv("AUTH0_CLIENT_SECRET"),
		Auth0Audience:          os.Getenv("AUTH0_AUDIENCE"),
		Auth0CallbackURL:       os.Getenv("AUTH0_CALLBACK_URL"),
		Auth0LogoutRedirectURL: os.Getenv("AUTH0_LOGOUT_REDIRECT_URL"),
		PostLoginRedirectURL:   os.Getenv("POST_LOGIN_REDIRECT_URL"),

		RedisURL:          os.Getenv("REDIS_URL"),
		SessionCookieName: getEnv("SESSION_COOKIE_NAME", "__Host-tdff-session"),

		BackendAPIBaseURL: os.Getenv("BACKEND_API_BASE_URL"),
	}

	// Default 7 days. This is a deliberate BFF-side policy choice, not one
	// derived from Auth0: the non-prod tenant's Application has "Set
	// Maximum Refresh Token Lifetime" disabled, so Auth0 imposes no
	// absolute cap of its own — refresh tokens there are effectively
	// indefinite. Combined with proxy.Handler re-saving the session (and
	// its full TTL) on every successful refresh, this value defines a
	// sliding idle-timeout window, not a hard expiry: an actively-used
	// session renews itself indefinitely, and only goes idle-expires after
	// this many seconds with no requests at all. Confirmed as the intended
	// behavior (not just an unexamined default) rather than adding an
	// absolute session-age cap on top.
	ttl, err := getEnvInt("SESSION_TTL_SECONDS", 60*60*24*7)
	if err != nil {
		return nil, err
	}
	cfg.SessionTTLSeconds = ttl

	if cfg.PostLoginRedirectURL == "" {
		cfg.PostLoginRedirectURL = cfg.Auth0LogoutRedirectURL
	}

	return cfg, nil
}

// RequireAuth0 fails fast if any Auth0 setting is missing. Call this before
// mounting the auth handlers (Phase 2).
func (c *Config) RequireAuth0() error {
	return requireAll(map[string]string{
		"AUTH0_DOMAIN":              c.Auth0Domain,
		"AUTH0_CLIENT_ID":           c.Auth0ClientID,
		"AUTH0_CLIENT_SECRET":       c.Auth0ClientSecret,
		"AUTH0_AUDIENCE":            c.Auth0Audience,
		"AUTH0_CALLBACK_URL":        c.Auth0CallbackURL,
		"AUTH0_LOGOUT_REDIRECT_URL": c.Auth0LogoutRedirectURL,
	})
}

// RequireRedis fails fast if Redis settings are missing. Call this before
// wiring the Redis-backed session store (Phase 3).
func (c *Config) RequireRedis() error {
	return requireAll(map[string]string{
		"REDIS_URL": c.RedisURL,
	})
}

// RequireBackendAPI fails fast if the proxy target is missing. Call this
// before mounting the proxy handler (Phase 4).
func (c *Config) RequireBackendAPI() error {
	return requireAll(map[string]string{
		"BACKEND_API_BASE_URL": c.BackendAPIBaseURL,
	})
}

// SessionCookieHasRecommendedPrefix reports whether SessionCookieName uses
// the __Host- prefix. This isn't just convention: browsers independently
// enforce Secure, no Domain attribute, and Path=/ for any cookie named with
// it, layering a browser-side guarantee on top of the attributes
// session.SetSessionCookie already sets explicitly. Non-fatal if false —
// callers should log a warning, not fail startup, since the cookie still
// gets the right attributes regardless of its name.
func (c *Config) SessionCookieHasRecommendedPrefix() bool {
	return strings.HasPrefix(c.SessionCookieName, "__Host-")
}

func requireAll(vars map[string]string) error {
	for name, val := range vars {
		if val == "" {
			return fmt.Errorf("config: required environment variable %s is not set", name)
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return 0, fmt.Errorf("config: %s must be an integer, got %q", key, v)
	}
	return n, nil
}
