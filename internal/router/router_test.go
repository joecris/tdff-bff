package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joecris/tdff-bff/internal/config"
	bffmw "github.com/joecris/tdff-bff/internal/middleware"
	"github.com/joecris/tdff-bff/internal/router"
)

func TestHealthz(t *testing.T) {
	cfg := &config.Config{AppEnv: "test"}
	h := router.New(router.Deps{Config: cfg})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body struct {
		Status string `json:"status"`
		Env    string `json:"env"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("expected status %q, got %q", "ok", body.Status)
	}
	if body.Env != "test" {
		t.Errorf("expected env %q, got %q", "test", body.Env)
	}
}

func TestHealthzSetsSecurityHeaders(t *testing.T) {
	h := router.New(router.Deps{Config: &config.Config{AppEnv: "test"}})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("expected security headers on a BFF-originated response like /healthz")
	}
}

// TestProxyRouteDoesNotDuplicateSecurityHeaders is a regression test: an
// earlier version applied bffmw.SecurityHeaders globally, which meant a
// proxied backend response ended up with two copies of headers like
// X-Frame-Options — ours, plus the backend's own (httputil.ReverseProxy
// appends the upstream's headers rather than replacing what's already on
// the ResponseWriter). SecurityHeaders must stay scoped to BFF-originated
// routes only.
func TestProxyRouteDoesNotDuplicateSecurityHeaders(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN") // the "backend"'s own header
		w.WriteHeader(http.StatusOK)
	})
	h := router.New(router.Deps{Config: &config.Config{AppEnv: "test"}, Proxy: stub})

	req := httptest.NewRequest(http.MethodGet, "/api/things", nil)
	req.Header.Set(bffmw.RequiredHeaderName, bffmw.RequiredHeaderValue)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	got := rec.Header().Values("X-Frame-Options")
	if len(got) != 1 || got[0] != "SAMEORIGIN" {
		t.Errorf("expected exactly the backend's own X-Frame-Options value, got %v", got)
	}
}

func TestProxyRouteRequiresCSRFHeader(t *testing.T) {
	var reached bool
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true })
	h := router.New(router.Deps{Config: &config.Config{AppEnv: "test"}, Proxy: stub})

	req := httptest.NewRequest(http.MethodGet, "/api/things", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 without the CSRF header, got %d", rec.Code)
	}
	if reached {
		t.Error("expected the proxy handler not to be reached without the CSRF header")
	}

	req = httptest.NewRequest(http.MethodGet, "/api/things", nil)
	req.Header.Set(bffmw.RequiredHeaderName, bffmw.RequiredHeaderValue)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !reached {
		t.Error("expected the proxy handler to be reached with the CSRF header present")
	}
}
