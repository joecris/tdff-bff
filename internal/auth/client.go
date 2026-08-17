// Package auth wraps the Auth0 side of the OAuth2/OIDC Authorization Code +
// PKCE flow: building the /authorize redirect, exchanging the returned code
// for tokens, verifying the ID token, refreshing an access token, and
// building the Auth0 logout URL. It builds on golang.org/x/oauth2 (PKCE
// helpers, token exchange) and coreos/go-oidc (discovery, ID token
// signature/issuer/audience/nonce verification) rather than hand-rolling
// either — both are the standard, widely-audited choices for this in Go.
package auth

import (
	"context"
	"fmt"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/joecris/tdff-bff/internal/config"
)

// Client is a configured Auth0 OIDC client, safe for concurrent use.
type Client struct {
	oauth2Config      *oauth2.Config
	verifier          *oidc.IDTokenVerifier
	domain            string
	audience          string
	logoutRedirectURL string
}

// NewClient discovers the tenant's OIDC configuration
// (.well-known/openid-configuration) and returns a ready-to-use Client.
// This makes a network call, so callers should apply a startup timeout via
// ctx and fail fast if the tenant is unreachable — better to know at boot
// than on the first login attempt.
func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	return newClientWithIssuer(ctx, "https://"+cfg.Auth0Domain+"/", cfg)
}

// newClientWithIssuer is NewClient with the issuer URL taken as a parameter
// instead of derived from cfg.Auth0Domain, so tests can point discovery at
// a local fake OIDC provider (plain http://, no real Auth0 tenant) without
// touching the network. Production always goes through NewClient.
func newClientWithIssuer(ctx context.Context, issuer string, cfg *config.Config) (*Client, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discover oidc provider at %s: %w", issuer, err)
	}

	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.Auth0ClientID,
		ClientSecret: cfg.Auth0ClientSecret,
		RedirectURL:  cfg.Auth0CallbackURL,
		Endpoint:     provider.Endpoint(),
		// offline_access is what makes Auth0 return a refresh token; it
		// also requires "Allow Offline Access" enabled on the Auth0 API
		// matching cfg.Auth0Audience.
		Scopes: []string{oidc.ScopeOpenID, "profile", "email", "offline_access"},
	}

	return &Client{
		oauth2Config:      oauth2Cfg,
		verifier:          provider.Verifier(&oidc.Config{ClientID: cfg.Auth0ClientID}),
		domain:            cfg.Auth0Domain,
		audience:          cfg.Auth0Audience,
		logoutRedirectURL: cfg.Auth0LogoutRedirectURL,
	}, nil
}

// Transaction holds the values generated for one login attempt that must
// round-trip through the browser (via session.Txn) and be checked against
// Auth0's response.
type Transaction struct {
	State        string
	Nonce        string
	CodeVerifier string
}

// NewTransaction generates a fresh state/nonce/PKCE-verifier set for one
// login attempt. oauth2.GenerateVerifier produces a cryptographically
// random, URL-safe string meeting PKCE's verifier requirements (RFC 7636);
// it's reused here for state and nonce too since both just need the same
// randomness property, not PKCE-specific semantics.
func NewTransaction() Transaction {
	return Transaction{
		State:        oauth2.GenerateVerifier(),
		Nonce:        oauth2.GenerateVerifier(),
		CodeVerifier: oauth2.GenerateVerifier(),
	}
}

// AuthCodeURL builds the Auth0 /authorize URL to redirect the browser to.
func (c *Client) AuthCodeURL(tx Transaction) string {
	opts := []oauth2.AuthCodeOption{
		oauth2.S256ChallengeOption(tx.CodeVerifier),
		oidc.Nonce(tx.Nonce),
		oauth2.SetAuthURLParam("audience", c.audience),
	}
	return c.oauth2Config.AuthCodeURL(tx.State, opts...)
}

// Result is what a successful Exchange yields: the raw OAuth2 token
// (access + refresh token) plus the claims pulled from the verified ID
// token.
type Result struct {
	Token   *oauth2.Token
	Subject string
	Email   string
}

// Exchange trades the authorization code for tokens, verifies the ID token
// (signature, issuer, audience, expiry via the library; nonce here), and
// returns the claims the session needs. code and tx come from the
// callback request and the matching Txn cookie respectively — the caller
// is responsible for having already checked tx.State against the request's
// state query parameter before calling this.
func (c *Client) Exchange(ctx context.Context, code string, tx Transaction) (*Result, error) {
	token, err := c.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(tx.CodeVerifier))
	if err != nil {
		return nil, fmt.Errorf("auth: exchange code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, fmt.Errorf("auth: token response had no id_token")
	}

	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("auth: verify id_token: %w", err)
	}
	if idToken.Nonce != tx.Nonce {
		return nil, fmt.Errorf("auth: id_token nonce mismatch")
	}

	var claims struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("auth: parse id_token claims: %w", err)
	}

	return &Result{Token: token, Subject: claims.Subject, Email: claims.Email}, nil
}

// Refresh exchanges a refresh token for a new access token.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	ts := c.oauth2Config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	token, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("auth: refresh token: %w", err)
	}
	return token, nil
}

// LogoutURL builds the Auth0 /v2/logout URL that ends the IdP-side session
// and returns the browser to the configured post-logout URL.
func (c *Client) LogoutURL() string {
	v := url.Values{}
	v.Set("client_id", c.oauth2Config.ClientID)
	v.Set("returnTo", c.logoutRedirectURL)
	return "https://" + c.domain + "/v2/logout?" + v.Encode()
}
