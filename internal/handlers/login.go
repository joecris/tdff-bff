package handlers

import (
	"log"
	"net/http"

	"github.com/joecris/tdff-bff/internal/auth"
	"github.com/joecris/tdff-bff/internal/session"
)

// Login starts the Authorization Code + PKCE flow: generates a fresh
// state/nonce/verifier, stashes them in the short-lived transaction cookie,
// and redirects the browser to Auth0.
func Login(authClient *auth.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tx := auth.NewTransaction()

		if err := session.SetTxnCookie(w, session.Txn{
			State:        tx.State,
			Nonce:        tx.Nonce,
			CodeVerifier: tx.CodeVerifier,
		}); err != nil {
			log.Printf("handlers: login: set txn cookie: %v", err)
			http.Error(w, "unable to start login", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, authClient.AuthCodeURL(tx), http.StatusFound)
	}
}
