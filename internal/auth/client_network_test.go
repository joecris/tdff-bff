package auth

import (
	"context"
	"testing"
	"time"

	"github.com/joecris/tdff-bff/internal/config"
)

func testConfigForFakeProvider() *config.Config {
	return &config.Config{
		Auth0Domain:            "unused.example.com", // only LogoutURL reads this; discovery uses the issuer param
		Auth0ClientID:          "client-id",
		Auth0ClientSecret:      "client-secret",
		Auth0Audience:          "https://api.example.com",
		Auth0CallbackURL:       "https://bff.example.com/bff/auth/callback",
		Auth0LogoutRedirectURL: "https://app.example.com",
	}
}

func TestNewClientWithIssuerDiscovery(t *testing.T) {
	fp := newFakeProvider(t)
	c, err := newClientWithIssuer(context.Background(), fp.issuer(), testConfigForFakeProvider())
	if err != nil {
		t.Fatalf("newClientWithIssuer: %v", err)
	}
	if c.oauth2Config.ClientID != "client-id" {
		t.Errorf("expected ClientID from config, got %q", c.oauth2Config.ClientID)
	}
	if c.oauth2Config.Endpoint.TokenURL != fp.issuer()+"/oauth/token" {
		t.Errorf("expected TokenURL populated from discovery, got %q", c.oauth2Config.Endpoint.TokenURL)
	}
}

func TestNewClientWithIssuerUnreachable(t *testing.T) {
	if _, err := newClientWithIssuer(context.Background(), "http://127.0.0.1:1", testConfigForFakeProvider()); err == nil {
		t.Fatal("expected an error when the issuer is unreachable")
	}
}

func TestExchangeSuccess(t *testing.T) {
	fp := newFakeProvider(t)
	c, err := newClientWithIssuer(context.Background(), fp.issuer(), testConfigForFakeProvider())
	if err != nil {
		t.Fatalf("newClientWithIssuer: %v", err)
	}

	tx := Transaction{State: "s", Nonce: "the-nonce", CodeVerifier: "the-verifier"}
	fp.respondWith(tokenResponseConfig{
		accessToken:    "access-token-value",
		refreshToken:   "refresh-token-value",
		expiresIn:      3600,
		includeIDToken: true,
		idTokenClaims: map[string]any{
			"iss":   fp.issuer(),
			"sub":   "user-123",
			"aud":   "client-id",
			"email": "user@example.com",
			"nonce": tx.Nonce,
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
		},
	})

	result, err := c.Exchange(context.Background(), "auth-code", tx)
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if result.Subject != "user-123" {
		t.Errorf("expected subject %q, got %q", "user-123", result.Subject)
	}
	if result.Email != "user@example.com" {
		t.Errorf("expected email %q, got %q", "user@example.com", result.Email)
	}
	if result.Token.AccessToken != "access-token-value" {
		t.Errorf("expected access token %q, got %q", "access-token-value", result.Token.AccessToken)
	}
	if result.Token.RefreshToken != "refresh-token-value" {
		t.Errorf("expected refresh token %q, got %q", "refresh-token-value", result.Token.RefreshToken)
	}
}

func TestExchangeNonceMismatch(t *testing.T) {
	fp := newFakeProvider(t)
	c, err := newClientWithIssuer(context.Background(), fp.issuer(), testConfigForFakeProvider())
	if err != nil {
		t.Fatalf("newClientWithIssuer: %v", err)
	}

	tx := Transaction{State: "s", Nonce: "expected-nonce", CodeVerifier: "v"}
	fp.respondWith(tokenResponseConfig{
		accessToken:    "at",
		expiresIn:      3600,
		includeIDToken: true,
		idTokenClaims: map[string]any{
			"iss":   fp.issuer(),
			"sub":   "user-123",
			"aud":   "client-id",
			"nonce": "different-nonce",
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
		},
	})

	if _, err := c.Exchange(context.Background(), "auth-code", tx); err == nil {
		t.Fatal("expected an error for a mismatched nonce")
	}
}

func TestExchangeMissingIDToken(t *testing.T) {
	fp := newFakeProvider(t)
	c, err := newClientWithIssuer(context.Background(), fp.issuer(), testConfigForFakeProvider())
	if err != nil {
		t.Fatalf("newClientWithIssuer: %v", err)
	}

	fp.respondWith(tokenResponseConfig{accessToken: "at", expiresIn: 3600, includeIDToken: false})

	if _, err := c.Exchange(context.Background(), "auth-code", Transaction{State: "s", Nonce: "n", CodeVerifier: "v"}); err == nil {
		t.Fatal("expected an error when the token response has no id_token")
	}
}

func TestExchangeTokenEndpointError(t *testing.T) {
	fp := newFakeProvider(t)
	c, err := newClientWithIssuer(context.Background(), fp.issuer(), testConfigForFakeProvider())
	if err != nil {
		t.Fatalf("newClientWithIssuer: %v", err)
	}

	fp.respondWith(tokenResponseConfig{statusCode: 400})

	if _, err := c.Exchange(context.Background(), "auth-code", Transaction{State: "s", Nonce: "n", CodeVerifier: "v"}); err == nil {
		t.Fatal("expected an error when the token endpoint fails")
	}
}

func TestRefreshSuccess(t *testing.T) {
	fp := newFakeProvider(t)
	c, err := newClientWithIssuer(context.Background(), fp.issuer(), testConfigForFakeProvider())
	if err != nil {
		t.Fatalf("newClientWithIssuer: %v", err)
	}

	fp.respondWith(tokenResponseConfig{
		accessToken:  "new-access-token",
		refreshToken: "new-refresh-token",
		expiresIn:    3600,
	})

	token, err := c.Refresh(context.Background(), "old-refresh-token")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if token.AccessToken != "new-access-token" {
		t.Errorf("expected access token %q, got %q", "new-access-token", token.AccessToken)
	}
	if token.RefreshToken != "new-refresh-token" {
		t.Errorf("expected refresh token %q, got %q", "new-refresh-token", token.RefreshToken)
	}
}

func TestRefreshFailure(t *testing.T) {
	fp := newFakeProvider(t)
	c, err := newClientWithIssuer(context.Background(), fp.issuer(), testConfigForFakeProvider())
	if err != nil {
		t.Fatalf("newClientWithIssuer: %v", err)
	}

	fp.respondWith(tokenResponseConfig{statusCode: 400})

	if _, err := c.Refresh(context.Background(), "bad-refresh-token"); err == nil {
		t.Fatal("expected an error when the refresh token is rejected")
	}
}
