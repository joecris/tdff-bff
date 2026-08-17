package auth

// This file is an internal (package auth) test file, not package auth_test:
// it needs to construct a Client directly from its unexported fields to
// test the pure, non-network methods (AuthCodeURL, LogoutURL) without
// paying for OIDC discovery via NewClient. The network-bound methods
// (NewClient, Exchange, Refresh) are covered separately in
// client_test.go against a fake OIDC provider.

import (
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func testClient() *Client {
	return &Client{
		oauth2Config: &oauth2.Config{
			ClientID:     "test-client-id",
			ClientSecret: "test-client-secret",
			RedirectURL:  "https://bff.example.com/bff/auth/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://tenant.auth0.com/authorize",
				TokenURL: "https://tenant.auth0.com/oauth/token",
			},
			Scopes: []string{"openid", "profile", "email", "offline_access"},
		},
		domain:            "tenant.auth0.com",
		audience:          "https://api.example.com",
		logoutRedirectURL: "https://app.example.com",
	}
}

func TestAuthCodeURL(t *testing.T) {
	c := testClient()
	tx := Transaction{State: "the-state", Nonce: "the-nonce", CodeVerifier: "the-verifier"}

	raw := c.AuthCodeURL(tx)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("AuthCodeURL produced an unparseable URL: %v", err)
	}
	if !strings.HasPrefix(raw, c.oauth2Config.Endpoint.AuthURL) {
		t.Errorf("expected URL to start with the authorize endpoint, got %q", raw)
	}

	q := u.Query()
	if q.Get("client_id") != c.oauth2Config.ClientID {
		t.Errorf("client_id: expected %q, got %q", c.oauth2Config.ClientID, q.Get("client_id"))
	}
	if q.Get("redirect_uri") != c.oauth2Config.RedirectURL {
		t.Errorf("redirect_uri: expected %q, got %q", c.oauth2Config.RedirectURL, q.Get("redirect_uri"))
	}
	if q.Get("state") != tx.State {
		t.Errorf("state: expected %q, got %q", tx.State, q.Get("state"))
	}
	if q.Get("nonce") != tx.Nonce {
		t.Errorf("nonce: expected %q, got %q", tx.Nonce, q.Get("nonce"))
	}
	if q.Get("audience") != c.audience {
		t.Errorf("audience: expected %q, got %q", c.audience, q.Get("audience"))
	}
	if q.Get("response_type") != "code" {
		t.Errorf("response_type: expected %q, got %q", "code", q.Get("response_type"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Errorf("code_challenge_method: expected S256, got %q", q.Get("code_challenge_method"))
	}
	if q.Get("code_challenge") == "" {
		t.Error("expected a non-empty code_challenge derived from the verifier")
	}
}

func TestLogoutURL(t *testing.T) {
	c := testClient()

	raw := c.LogoutURL()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("LogoutURL produced an unparseable URL: %v", err)
	}
	if !strings.HasPrefix(raw, "https://"+c.domain+"/v2/logout") {
		t.Errorf("expected URL to start with the tenant's /v2/logout, got %q", raw)
	}

	q := u.Query()
	if q.Get("client_id") != c.oauth2Config.ClientID {
		t.Errorf("client_id: expected %q, got %q", c.oauth2Config.ClientID, q.Get("client_id"))
	}
	if q.Get("returnTo") != c.logoutRedirectURL {
		t.Errorf("returnTo: expected %q, got %q", c.logoutRedirectURL, q.Get("returnTo"))
	}
}
