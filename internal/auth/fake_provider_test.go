package auth

// Internal (package auth) test helper: a minimal fake OIDC provider
// (discovery doc + JWKS + token endpoint) so NewClient/Exchange/Refresh can
// be tested against real OIDC discovery/verification machinery without a
// live Auth0 tenant. Kept deliberately small — just enough surface for
// go-oidc's provider discovery and ID token verification to succeed.

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-jose/go-jose/v4"
)

const fakeProviderKeyID = "test-key"

type tokenResponseConfig struct {
	accessToken    string
	refreshToken   string
	expiresIn      int
	includeIDToken bool
	idTokenClaims  map[string]any
	statusCode     int // 0 defaults to 200
}

type fakeProvider struct {
	server            *httptest.Server
	privateKey        *rsa.PrivateKey
	nextTokenResponse *tokenResponseConfig
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	fp := &fakeProvider{privateKey: key}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", fp.handleDiscovery)
	mux.HandleFunc("/jwks", fp.handleJWKS)
	mux.HandleFunc("/oauth/token", fp.handleToken)

	fp.server = httptest.NewServer(mux)
	t.Cleanup(fp.server.Close)
	return fp
}

func (fp *fakeProvider) issuer() string { return fp.server.URL }

// respondWith configures what the next (and only expected) call to
// /oauth/token returns.
func (fp *fakeProvider) respondWith(cfg tokenResponseConfig) {
	fp.nextTokenResponse = &cfg
}

func (fp *fakeProvider) handleDiscovery(w http.ResponseWriter, _ *http.Request) {
	doc := map[string]any{
		"issuer":                                fp.issuer(),
		"authorization_endpoint":                fp.issuer() + "/authorize",
		"token_endpoint":                        fp.issuer() + "/oauth/token",
		"jwks_uri":                              fp.issuer() + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(doc)
}

func (fp *fakeProvider) handleJWKS(w http.ResponseWriter, _ *http.Request) {
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
		Key:       &fp.privateKey.PublicKey,
		KeyID:     fakeProviderKeyID,
		Algorithm: "RS256",
		Use:       "sig",
	}}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(set)
}

func (fp *fakeProvider) handleToken(w http.ResponseWriter, _ *http.Request) {
	cfg := fp.nextTokenResponse
	if cfg == nil {
		http.Error(w, "test: no token response configured", http.StatusInternalServerError)
		return
	}

	if cfg.statusCode != 0 && cfg.statusCode != http.StatusOK {
		w.WriteHeader(cfg.statusCode)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "test_configured_error"})
		return
	}

	resp := map[string]any{
		"access_token":  cfg.accessToken,
		"refresh_token": cfg.refreshToken,
		"token_type":    "Bearer",
		"expires_in":    cfg.expiresIn,
	}
	if cfg.includeIDToken {
		idToken, err := fp.signIDToken(cfg.idTokenClaims)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp["id_token"] = idToken
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (fp *fakeProvider) signIDToken(claims map[string]any) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: fp.privateKey},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader(jose.HeaderKey("kid"), fakeProviderKeyID),
	)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	jws, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return jws.CompactSerialize()
}
