package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joecris/tdff-bff/internal/handlers"
	"github.com/joecris/tdff-bff/internal/session"
)

func TestLoginRedirectsToAuthCodeURLAndSetsTxnCookie(t *testing.T) {
	fake := &fakeOIDCClient{authCodeURL: "https://idp.example.com/authorize?state=xyz"}
	h := handlers.Login(fake)

	req := httptest.NewRequest(http.MethodGet, "/bff/auth/login", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != fake.authCodeURL {
		t.Errorf("expected redirect to %q, got %q", fake.authCodeURL, loc)
	}

	var found bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "__Host-tdff-auth-txn" {
			found = true
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
				t.Errorf("txn cookie attributes unexpected: %+v", c)
			}
		}
	}
	if !found {
		t.Error("expected the auth transaction cookie to be set")
	}
}

// txnFromResponse extracts and decodes the Txn cookie set on rec's
// response, so tests can inspect what Login actually stored.
func txnFromResponse(t *testing.T, rec *httptest.ResponseRecorder) session.Txn {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/bff/auth/callback", nil)
	for _, c := range rec.Result().Cookies() {
		req.AddCookie(c)
	}
	tx, err := session.ReadTxnCookie(req)
	if err != nil {
		t.Fatalf("ReadTxnCookie: %v", err)
	}
	return tx
}

func TestLoginWithValidReturnToStoresIt(t *testing.T) {
	fake := &fakeOIDCClient{authCodeURL: "https://idp.example.com/authorize"}
	h := handlers.Login(fake)

	req := httptest.NewRequest(http.MethodGet, "/bff/auth/login?returnTo=%2Fleagues%2F123", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := txnFromResponse(t, rec).ReturnTo; got != "/leagues/123" {
		t.Errorf("expected ReturnTo %q, got %q", "/leagues/123", got)
	}
}

func TestLoginWithUnsafeReturnToDropsIt(t *testing.T) {
	fake := &fakeOIDCClient{authCodeURL: "https://idp.example.com/authorize"}
	h := handlers.Login(fake)

	req := httptest.NewRequest(http.MethodGet, "/bff/auth/login?returnTo=https%3A%2F%2Fevil.com", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := txnFromResponse(t, rec).ReturnTo; got != "" {
		t.Errorf("expected an unsafe returnTo to be dropped, got %q", got)
	}
}
