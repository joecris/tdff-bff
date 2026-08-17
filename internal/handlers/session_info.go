package handlers

import (
	"net/http"

	"github.com/joecris/tdff-bff/internal/config"
	"github.com/joecris/tdff-bff/internal/session"
)

type sessionInfoResponse struct {
	Authenticated bool   `json:"authenticated"`
	Email         string `json:"email,omitempty"`
}

// SessionInfo answers the SPA's "am I logged in" check. It deliberately
// never returns token material — only enough to drive UI state.
func SessionInfo(store session.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := session.ReadSessionCookie(r, cfg)
		if err != nil {
			writeJSON(w, http.StatusOK, sessionInfoResponse{Authenticated: false})
			return
		}

		data, err := store.Get(r.Context(), sessionID)
		if err != nil {
			writeJSON(w, http.StatusOK, sessionInfoResponse{Authenticated: false})
			return
		}

		writeJSON(w, http.StatusOK, sessionInfoResponse{Authenticated: true, Email: data.Email})
	}
}
