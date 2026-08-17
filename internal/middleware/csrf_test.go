package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joecris/tdff-bff/internal/middleware"
)

func TestRequireCustomHeaderMissing(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	h := middleware.RequireCustomHeader(next)

	req := httptest.NewRequest(http.MethodGet, "/api/things", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
	if called {
		t.Error("expected next handler not to be called")
	}
}

func TestRequireCustomHeaderWrongValue(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h := middleware.RequireCustomHeader(next)

	req := httptest.NewRequest(http.MethodGet, "/api/things", nil)
	req.Header.Set(middleware.RequiredHeaderName, "something-else")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequireCustomHeaderPresent(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	h := middleware.RequireCustomHeader(next)

	req := httptest.NewRequest(http.MethodGet, "/api/things", nil)
	req.Header.Set(middleware.RequiredHeaderName, middleware.RequiredHeaderValue)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Error("expected next handler to be called")
	}
}
