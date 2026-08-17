package config_test

import (
	"testing"

	"github.com/joecris/tdff-bff/internal/config"
)

// knownVars is every env var config.Load reads. Tests clear all of them
// first (via t.Setenv to "", which our getEnv/os.Getenv treats the same as
// unset) so a developer's real shell environment can't leak into a test.
var knownVars = []string{
	"APP_ENV", "PORT", "LOG_LEVEL",
	"AUTH0_DOMAIN", "AUTH0_CLIENT_ID", "AUTH0_CLIENT_SECRET", "AUTH0_AUDIENCE",
	"AUTH0_CALLBACK_URL", "AUTH0_LOGOUT_REDIRECT_URL", "POST_LOGIN_REDIRECT_URL",
	"REDIS_URL", "SESSION_COOKIE_NAME", "SESSION_TTL_SECONDS",
	"BACKEND_API_BASE_URL",
}

func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range knownVars {
		t.Setenv(k, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("AppEnv: expected %q, got %q", "development", cfg.AppEnv)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port: expected %q, got %q", "8080", cfg.Port)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: expected %q, got %q", "info", cfg.LogLevel)
	}
	if cfg.SessionCookieName != "__Host-tdff-session" {
		t.Errorf("SessionCookieName: expected default, got %q", cfg.SessionCookieName)
	}
	if cfg.SessionTTLSeconds != 60*60*24*7 {
		t.Errorf("SessionTTLSeconds: expected 604800, got %d", cfg.SessionTTLSeconds)
	}
	if cfg.PostLoginRedirectURL != "" {
		t.Errorf("PostLoginRedirectURL: expected empty (both sources unset), got %q", cfg.PostLoginRedirectURL)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("AUTH0_DOMAIN", "example.auth0.com")
	t.Setenv("AUTH0_CLIENT_ID", "client-id")
	t.Setenv("AUTH0_CLIENT_SECRET", "client-secret")
	t.Setenv("AUTH0_AUDIENCE", "https://api.example.com")
	t.Setenv("AUTH0_CALLBACK_URL", "https://bff.example.com/bff/auth/callback")
	t.Setenv("AUTH0_LOGOUT_REDIRECT_URL", "https://app.example.com")
	t.Setenv("REDIS_URL", "rediss://user:pass@redis.example.com:6379")
	t.Setenv("SESSION_COOKIE_NAME", "__Host-custom")
	t.Setenv("SESSION_TTL_SECONDS", "120")
	t.Setenv("BACKEND_API_BASE_URL", "https://api.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	cases := map[string]struct{ got, want string }{
		"AppEnv":                 {cfg.AppEnv, "prod"},
		"Port":                   {cfg.Port, "9090"},
		"LogLevel":               {cfg.LogLevel, "debug"},
		"Auth0Domain":            {cfg.Auth0Domain, "example.auth0.com"},
		"Auth0ClientID":          {cfg.Auth0ClientID, "client-id"},
		"Auth0ClientSecret":      {cfg.Auth0ClientSecret, "client-secret"},
		"Auth0Audience":          {cfg.Auth0Audience, "https://api.example.com"},
		"Auth0CallbackURL":       {cfg.Auth0CallbackURL, "https://bff.example.com/bff/auth/callback"},
		"Auth0LogoutRedirectURL": {cfg.Auth0LogoutRedirectURL, "https://app.example.com"},
		"RedisURL":               {cfg.RedisURL, "rediss://user:pass@redis.example.com:6379"},
		"SessionCookieName":      {cfg.SessionCookieName, "__Host-custom"},
		"BackendAPIBaseURL":      {cfg.BackendAPIBaseURL, "https://api.example.com"},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: expected %q, got %q", name, c.want, c.got)
		}
	}
	if cfg.SessionTTLSeconds != 120 {
		t.Errorf("SessionTTLSeconds: expected 120, got %d", cfg.SessionTTLSeconds)
	}
}

func TestLoadPostLoginRedirectDefaultsToLogoutRedirect(t *testing.T) {
	clearEnv(t)
	t.Setenv("AUTH0_LOGOUT_REDIRECT_URL", "https://app.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PostLoginRedirectURL != "https://app.example.com" {
		t.Errorf("expected PostLoginRedirectURL to default to logout redirect, got %q", cfg.PostLoginRedirectURL)
	}
}

func TestLoadPostLoginRedirectExplicitOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("AUTH0_LOGOUT_REDIRECT_URL", "https://app.example.com")
	t.Setenv("POST_LOGIN_REDIRECT_URL", "https://app.example.com/dashboard")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PostLoginRedirectURL != "https://app.example.com/dashboard" {
		t.Errorf("expected explicit PostLoginRedirectURL to win, got %q", cfg.PostLoginRedirectURL)
	}
}

func TestLoadInvalidSessionTTL(t *testing.T) {
	clearEnv(t)
	t.Setenv("SESSION_TTL_SECONDS", "not-a-number")

	if _, err := config.Load(); err == nil {
		t.Fatal("expected an error for a non-integer SESSION_TTL_SECONDS")
	}
}

func TestRequireAuth0(t *testing.T) {
	full := config.Config{
		Auth0Domain:            "d",
		Auth0ClientID:          "id",
		Auth0ClientSecret:      "secret",
		Auth0Audience:          "aud",
		Auth0CallbackURL:       "cb",
		Auth0LogoutRedirectURL: "logout",
	}
	if err := full.RequireAuth0(); err != nil {
		t.Errorf("expected no error with all fields set, got %v", err)
	}

	missingDomain := full
	missingDomain.Auth0Domain = ""
	if err := missingDomain.RequireAuth0(); err == nil {
		t.Error("expected an error with Auth0Domain missing")
	}
}

func TestRequireRedis(t *testing.T) {
	if err := (&config.Config{RedisURL: "redis://localhost:6379"}).RequireRedis(); err != nil {
		t.Errorf("expected no error with RedisURL set, got %v", err)
	}
	if err := (&config.Config{}).RequireRedis(); err == nil {
		t.Error("expected an error with RedisURL missing")
	}
}

func TestRequireBackendAPI(t *testing.T) {
	if err := (&config.Config{BackendAPIBaseURL: "http://localhost:3000"}).RequireBackendAPI(); err != nil {
		t.Errorf("expected no error with BackendAPIBaseURL set, got %v", err)
	}
	if err := (&config.Config{}).RequireBackendAPI(); err == nil {
		t.Error("expected an error with BackendAPIBaseURL missing")
	}
}
