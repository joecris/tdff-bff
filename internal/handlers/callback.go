package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/joecris/tdff-bff/internal/auth"
	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/session"
)

// Callback completes the flow Login started: validates the transaction
// cookie and returned state, exchanges the code for tokens, verifies the ID
// token, creates a session, and redirects the browser back to the SPA.
func Callback(authClient OIDCClient, store session.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if errCode := q.Get("error"); errCode != "" {
			clientError(w, http.StatusBadRequest, "authentication failed",
				"callback: auth0 returned an error",
				fmt.Errorf("%s: %s", errCode, q.Get("error_description")))
			return
		}

		tx, err := session.ReadTxnCookie(r)
		if err != nil {
			clientError(w, http.StatusBadRequest, "login session expired, please try again",
				"callback: read txn cookie", err)
			return
		}
		session.ClearTxnCookie(w) // single-use, consume it regardless of outcome below

		if state := q.Get("state"); state == "" || state != tx.State {
			clientError(w, http.StatusBadRequest, "authentication failed",
				"callback: state mismatch", fmt.Errorf("got %q", state))
			return
		}

		code := q.Get("code")
		if code == "" {
			clientError(w, http.StatusBadRequest, "authentication failed",
				"callback: missing code", fmt.Errorf("no code in callback query"))
			return
		}

		result, err := authClient.Exchange(r.Context(), code, auth.Transaction{
			State:        tx.State,
			Nonce:        tx.Nonce,
			CodeVerifier: tx.CodeVerifier,
		})
		if err != nil {
			clientError(w, http.StatusBadGateway, "authentication failed",
				"callback: exchange", err)
			return
		}

		sessionID := session.NewID()
		data := session.Data{
			Subject:      result.Subject,
			Email:        result.Email,
			AccessToken:  result.Token.AccessToken,
			RefreshToken: result.Token.RefreshToken,
			ExpiresAt:    result.Token.Expiry,
		}
		ttl := time.Duration(cfg.SessionTTLSeconds) * time.Second
		if err := store.Save(r.Context(), sessionID, data, ttl); err != nil {
			clientError(w, http.StatusInternalServerError, "authentication failed",
				"callback: save session", err)
			return
		}

		// Diagnostic, not throwaway: a missing refresh token here means
		// "Allow Offline Access" isn't enabled on the Auth0 API, and
		// refresh handling (a core BFF responsibility) will silently break
		// the moment the access token expires. Cheap to keep permanently.
		log.Printf("handlers: callback: session created for %s (refresh_token_present=%t)",
			data.Email, data.RefreshToken != "")

		session.SetSessionCookie(w, sessionID, cfg)

		// tx.ReturnTo was already validated by session.SanitizeReturnTo
		// back in Login, before it was ever written to the (HttpOnly,
		// Secure, single-use) txn cookie — trusted as-is here.
		redirectTo := cfg.PostLoginRedirectURL
		if tx.ReturnTo != "" {
			redirectTo = tx.ReturnTo
		}
		http.Redirect(w, r, redirectTo, http.StatusFound)
	}
}
