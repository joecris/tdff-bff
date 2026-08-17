package handlers_test

import (
	"context"

	"github.com/joecris/tdff-bff/internal/auth"
)

// fakeOIDCClient implements handlers.OIDCClient without touching a real
// Auth0 tenant, so the handler tests can exercise every branch (including
// error paths that are impractical to trigger against a live IdP).
type fakeOIDCClient struct {
	authCodeURL string
	logoutURL   string

	exchangeResult  *auth.Result
	exchangeErr     error
	gotExchangeTx   auth.Transaction
	gotExchangeCode string
}

func (f *fakeOIDCClient) AuthCodeURL(auth.Transaction) string { return f.authCodeURL }

func (f *fakeOIDCClient) Exchange(_ context.Context, code string, tx auth.Transaction) (*auth.Result, error) {
	f.gotExchangeCode = code
	f.gotExchangeTx = tx
	return f.exchangeResult, f.exchangeErr
}

func (f *fakeOIDCClient) LogoutURL() string { return f.logoutURL }
