package handlers

import (
	"log"
	"net/http"

	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/session"
)

// Logout deletes the server-side session (revoking it — the RFC's
// server-side-session model means this is enough, no separate token
// revocation call needed), clears the cookie, and sends the browser to
// Auth0's /v2/logout so the IdP-side SSO session ends too.
func Logout(authClient OIDCClient, store session.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sessionID, err := session.ReadSessionCookie(r, cfg); err == nil {
			if err := store.Delete(r.Context(), sessionID); err != nil {
				log.Printf("handlers: logout: delete session: %v", err)
				// Not fatal to the logout UX: still clear the cookie and
				// send the browser on. Worst case a stale Redis entry
				// expires on its own TTL.
			}
		}

		session.ClearSessionCookie(w, cfg)
		http.Redirect(w, r, authClient.LogoutURL(), http.StatusFound)
	}
}
