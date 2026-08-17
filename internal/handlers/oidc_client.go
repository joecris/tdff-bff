package handlers

import (
	"context"

	"github.com/joecris/tdff-bff/internal/auth"
)

// OIDCClient is the subset of auth.Client's behavior these handlers depend
// on. Declaring it here — rather than taking the concrete type — lets the
// handlers be unit tested with a fake, without a live Auth0 tenant.
// *auth.Client satisfies this implicitly.
type OIDCClient interface {
	AuthCodeURL(tx auth.Transaction) string
	Exchange(ctx context.Context, code string, tx auth.Transaction) (*auth.Result, error)
	LogoutURL() string
}
