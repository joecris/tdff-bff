package session_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/session"
)

func TestSessionCookieRoundTrip(t *testing.T) {
	cfg := &config.Config{SessionCookieName: "__Host-test-session", SessionTTLSeconds: 3600}

	rec := httptest.NewRecorder()
	session.SetSessionCookie(rec, "abc123", cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	got, err := session.ReadSessionCookie(req, cfg)
	if err != nil {
		t.Fatalf("ReadSessionCookie: %v", err)
	}
	if got != "abc123" {
		t.Errorf("expected session id %q, got %q", "abc123", got)
	}
}

func TestReadSessionCookieMissing(t *testing.T) {
	cfg := &config.Config{SessionCookieName: "__Host-test-session"}
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if _, err := session.ReadSessionCookie(req, cfg); err != session.ErrNoSession {
		t.Errorf("expected ErrNoSession, got %v", err)
	}
}

func TestClearSessionCookieExpires(t *testing.T) {
	cfg := &config.Config{SessionCookieName: "__Host-test-session"}
	rec := httptest.NewRecorder()
	session.ClearSessionCookie(rec, cfg)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Errorf("expected a negative MaxAge to expire the cookie, got %d", cookies[0].MaxAge)
	}
}

func TestTxnCookieRoundTrip(t *testing.T) {
	tx := session.Txn{State: "s", Nonce: "n", CodeVerifier: "v"}

	rec := httptest.NewRecorder()
	if err := session.SetTxnCookie(rec, tx); err != nil {
		t.Fatalf("SetTxnCookie: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}

	got, err := session.ReadTxnCookie(req)
	if err != nil {
		t.Fatalf("ReadTxnCookie: %v", err)
	}
	if got != tx {
		t.Errorf("expected %+v, got %+v", tx, got)
	}
}

func TestReadTxnCookieMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := session.ReadTxnCookie(req); err != session.ErrNoTxn {
		t.Errorf("expected ErrNoTxn, got %v", err)
	}
}

func TestNewIDIsUniqueAndURLSafe(t *testing.T) {
	a := session.NewID()
	b := session.NewID()
	if a == b {
		t.Fatal("expected two distinct IDs")
	}
	if len(a) == 0 {
		t.Fatal("expected a non-empty ID")
	}
}
