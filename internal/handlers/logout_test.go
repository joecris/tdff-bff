package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joecris/tdff-bff/internal/handlers"
	"github.com/joecris/tdff-bff/internal/session"
	"github.com/joecris/tdff-bff/internal/store/memory"
)

func TestLogoutWithActiveSessionDeletesItAndRedirects(t *testing.T) {
	cfg := testConfig()
	store := memory.New()
	sessionID := "sess-1"
	if err := store.Save(context.Background(), sessionID, session.Data{Email: "user@example.com"}, time.Hour); err != nil {
		t.Fatalf("Save: %v", err)
	}

	fake := &fakeOIDCClient{logoutURL: "https://idp.example.com/v2/logout?returnTo=..."}
	h := handlers.Logout(fake, store, cfg)

	rec0 := httptest.NewRecorder()
	session.SetSessionCookie(rec0, sessionID, cfg)
	req := httptest.NewRequest(http.MethodGet, "/bff/auth/logout", nil)
	for _, c := range rec0.Result().Cookies() {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != fake.logoutURL {
		t.Errorf("expected redirect to %q, got %q", fake.logoutURL, loc)
	}

	if _, err := store.Get(context.Background(), sessionID); err != session.ErrNotFound {
		t.Errorf("expected session to be deleted, got err=%v", err)
	}

	var cleared bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == cfg.SessionCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("expected the session cookie to be cleared")
	}
}

func TestLogoutWithNoSessionStillRedirects(t *testing.T) {
	cfg := testConfig()
	fake := &fakeOIDCClient{logoutURL: "https://idp.example.com/v2/logout"}
	h := handlers.Logout(fake, memory.New(), cfg)

	req := httptest.NewRequest(http.MethodGet, "/bff/auth/logout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != fake.logoutURL {
		t.Errorf("expected redirect to %q, got %q", fake.logoutURL, loc)
	}
}
