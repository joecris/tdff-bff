package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joecris/tdff-bff/internal/handlers"
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
