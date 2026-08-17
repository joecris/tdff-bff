package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joecris/tdff-bff/internal/middleware"
)

func TestSecurityHeadersSetsExpectedHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := middleware.SecurityHeaders(next)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "no-referrer",
		"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
	}
	for header, expected := range want {
		if got := rec.Header().Get(header); got != expected {
			t.Errorf("%s: expected %q, got %q", header, expected, got)
		}
	}
	if rec.Header().Get("Permissions-Policy") == "" {
		t.Error("expected a non-empty Permissions-Policy header")
	}
}
