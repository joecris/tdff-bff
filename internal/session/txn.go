package session

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

// txnCookieName does not use cfg.SessionCookieName's prefix scheme on
// purpose: this is a separate, short-lived cookie distinct from the session
// cookie, alive only for the duration of the Auth0 redirect round trip.
const txnCookieName = "__Host-tdff-auth-txn"

const txnTTL = 10 * time.Minute

// Txn carries the values generated at /bff/auth/login that must be checked
// against Auth0's response at /bff/auth/callback: state (CSRF protection for
// the redirect), nonce (replay protection for the ID token), and the PKCE
// code verifier. It isn't signed or encrypted: HttpOnly+Secure already
// prevents both JS and network tampering, and the values are single-use,
// short-lived, and meaningless outside a comparison against Auth0's
// response — encryption would add complexity without closing a real gap.
//
// ReturnTo is different in kind from the other three: it's not part of the
// OIDC exchange, just where Callback sends the browser on success if
// non-empty (falling back to cfg.PostLoginRedirectURL otherwise). Already
// validated by SanitizeReturnTo before it's ever stored here — Callback
// trusts it as-is rather than re-validating, since this cookie is
// HttpOnly/Secure and single-use.
type Txn struct {
	State        string `json:"state"`
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"code_verifier"`
	ReturnTo     string `json:"return_to,omitempty"`
}

// SetTxnCookie stores tx for the callback leg of the flow.
//
// SameSite=Lax, not Strict: Auth0 redirects the browser back to
// /bff/auth/callback via a top-level cross-site navigation (the request
// originates from auth0.com). Strict cookies are withheld on cross-site
// navigations, which would make this cookie invisible exactly when the
// callback handler needs it. Lax still permits it on top-level GET
// navigations while still refusing it on cross-site subrequests (images,
// fetches, etc.), which is the property we actually need here.
func SetTxnCookie(w http.ResponseWriter, tx Txn) error {
	b, err := json.Marshal(tx)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     txnCookieName,
		Value:    base64.RawURLEncoding.EncodeToString(b),
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(txnTTL.Seconds()),
	})
	return nil
}

// ReadTxnCookie recovers the Txn stored by SetTxnCookie.
func ReadTxnCookie(r *http.Request) (Txn, error) {
	c, err := r.Cookie(txnCookieName)
	if err != nil || c.Value == "" {
		return Txn{}, ErrNoTxn
	}
	b, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return Txn{}, ErrNoTxn
	}
	var tx Txn
	if err := json.Unmarshal(b, &tx); err != nil {
		return Txn{}, ErrNoTxn
	}
	return tx, nil
}

// ClearTxnCookie expires the transaction cookie once the callback has
// consumed it (successfully or not) — it's single-use.
func ClearTxnCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     txnCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
