package handlers

import (
	"log"
	"net/http"

	"github.com/joecris/tdff-bff/internal/auth"
	"github.com/joecris/tdff-bff/internal/session"
)

// Login starts the Authorization Code + PKCE flow: generates a fresh
// state/nonce/verifier, stashes them (plus an optional, validated
// ?returnTo= path) in the short-lived transaction cookie, and redirects the
// browser to Auth0.
//
// ?returnTo lets the SPA send the browser back to wherever it started
// (e.g. a deep link) instead of always landing on the fixed
// PostLoginRedirectURL — see session.SanitizeReturnTo for why raw values
// aren't trusted as-is. An invalid returnTo is silently dropped (falls
// back to the default landing page) rather than erroring the whole login
// attempt over it.
func Login(authClient OIDCClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tx := auth.NewTransaction()
		returnTo := session.SanitizeReturnTo(r.URL.Query().Get("returnTo"))

		if err := session.SetTxnCookie(w, session.Txn{
			State:        tx.State,
			Nonce:        tx.Nonce,
			CodeVerifier: tx.CodeVerifier,
			ReturnTo:     returnTo,
		}); err != nil {
			log.Printf("handlers: login: set txn cookie: %v", err)
			http.Error(w, "unable to start login", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, authClient.AuthCodeURL(tx), http.StatusFound)
	}
}
