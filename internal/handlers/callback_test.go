package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/joecris/tdff-bff/internal/auth"
	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/handlers"
	"github.com/joecris/tdff-bff/internal/session"
	"github.com/joecris/tdff-bff/internal/store/memory"
)

func testConfig() *config.Config {
	return &config.Config{
		SessionCookieName:    "__Host-test-session",
		SessionTTLSeconds:    3600,
		PostLoginRedirectURL: "https://app.example.com",
	}
}

// requestWithTxn builds a callback request carrying a valid txn cookie for
// the given state/nonce/verifier, so tests can focus on one branch at a
// time instead of re-deriving cookie plumbing everywhere.
func requestWithTxn(t *testing.T, target string, tx session.Txn) *http.Request {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := session.SetTxnCookie(rec, tx); err != nil {
		t.Fatalf("SetTxnCookie: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	return req
}

func TestCallbackAuth0Error(t *testing.T) {
	cfg := testConfig()
	h := handlers.Callback(&fakeOIDCClient{}, memory.New(), cfg)

	req := httptest.NewRequest(http.MethodGet, "/bff/auth/callback?error=access_denied&error_description=nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCallbackMissingTxnCookie(t *testing.T) {
	cfg := testConfig()
	h := handlers.Callback(&fakeOIDCClient{}, memory.New(), cfg)

	req := httptest.NewRequest(http.MethodGet, "/bff/auth/callback?code=abc&state=xyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCallbackStateMismatch(t *testing.T) {
	cfg := testConfig()
	h := handlers.Callback(&fakeOIDCClient{}, memory.New(), cfg)

	req := requestWithTxn(t, "/bff/auth/callback?code=abc&state=WRONG", session.Txn{State: "expected-state"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCallbackMissingCode(t *testing.T) {
	cfg := testConfig()
	h := handlers.Callback(&fakeOIDCClient{}, memory.New(), cfg)

	req := requestWithTxn(t, "/bff/auth/callback?state=s", session.Txn{State: "s"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCallbackExchangeFailure(t *testing.T) {
	cfg := testConfig()
	fake := &fakeOIDCClient{exchangeErr: errBoom}
	h := handlers.Callback(fake, memory.New(), cfg)

	req := requestWithTxn(t, "/bff/auth/callback?code=abc&state=s", session.Txn{State: "s"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", rec.Code)
	}
}

func TestCallbackSuccess(t *testing.T) {
	cfg := testConfig()
	store := memory.New()
	fake := &fakeOIDCClient{exchangeResult: &auth.Result{
		Subject: "user-1",
		Email:   "user@example.com",
		Token: &oauth2.Token{
			AccessToken:  "at",
			RefreshToken: "rt",
			Expiry:       time.Now().Add(time.Hour),
		},
	}}
	h := handlers.Callback(fake, store, cfg)

	req := requestWithTxn(t, "/bff/auth/callback?code=the-code&state=s", session.Txn{
		State: "s", Nonce: "n", CodeVerifier: "v",
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != cfg.PostLoginRedirectURL {
		t.Errorf("expected redirect to %q, got %q", cfg.PostLoginRedirectURL, loc)
	}
	if fake.gotExchangeCode != "the-code" {
		t.Errorf("expected code %q passed to Exchange, got %q", "the-code", fake.gotExchangeCode)
	}
	if fake.gotExchangeTx.Nonce != "n" || fake.gotExchangeTx.CodeVerifier != "v" {
		t.Errorf("expected txn values passed to Exchange, got %+v", fake.gotExchangeTx)
	}

	var sessionID string
	for _, c := range rec.Result().Cookies() {
		if c.Name == cfg.SessionCookieName {
			sessionID = c.Value
		}
	}
	if sessionID == "" {
		t.Fatal("expected a session cookie to be set")
	}

	data, err := store.Get(req.Context(), sessionID)
	if err != nil {
		t.Fatalf("expected session to be saved: %v", err)
	}
	if data.Email != "user@example.com" || data.AccessToken != "at" || data.RefreshToken != "rt" {
		t.Errorf("unexpected saved session data: %+v", data)
	}
}
