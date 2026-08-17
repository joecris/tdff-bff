package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joecris/tdff-bff/internal/handlers"
	"github.com/joecris/tdff-bff/internal/session"
	"github.com/joecris/tdff-bff/internal/store/memory"
)

func decodeSessionInfo(t *testing.T, rec *httptest.ResponseRecorder) (authenticated bool, email string) {
	t.Helper()
	var body struct {
		Authenticated bool   `json:"authenticated"`
		Email         string `json:"email"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body.Authenticated, body.Email
}

func TestSessionInfoNoCookie(t *testing.T) {
	cfg := testConfig()
	h := handlers.SessionInfo(memory.New(), cfg)

	req := httptest.NewRequest(http.MethodGet, "/bff/auth/session", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	authenticated, _ := decodeSessionInfo(t, rec)
	if authenticated {
		t.Error("expected authenticated=false with no cookie")
	}
}

func TestSessionInfoUnknownSession(t *testing.T) {
	cfg := testConfig()
	h := handlers.SessionInfo(memory.New(), cfg)

	rec0 := httptest.NewRecorder()
	session.SetSessionCookie(rec0, "does-not-exist", cfg)
	req := httptest.NewRequest(http.MethodGet, "/bff/auth/session", nil)
	for _, c := range rec0.Result().Cookies() {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	authenticated, _ := decodeSessionInfo(t, rec)
	if authenticated {
		t.Error("expected authenticated=false for an unknown session")
	}
}

func TestSessionInfoActiveSession(t *testing.T) {
	cfg := testConfig()
	store := memory.New()
	sessionID := "sess-1"
	if err := store.Save(context.Background(), sessionID, session.Data{Email: "user@example.com"}, time.Hour); err != nil {
		t.Fatalf("Save: %v", err)
	}
	h := handlers.SessionInfo(store, cfg)

	rec0 := httptest.NewRecorder()
	session.SetSessionCookie(rec0, sessionID, cfg)
	req := httptest.NewRequest(http.MethodGet, "/bff/auth/session", nil)
	for _, c := range rec0.Result().Cookies() {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	authenticated, email := decodeSessionInfo(t, rec)
	if !authenticated {
		t.Error("expected authenticated=true for an active session")
	}
	if email != "user@example.com" {
		t.Errorf("expected email %q, got %q", "user@example.com", email)
	}
}
