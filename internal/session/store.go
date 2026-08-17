// Package session owns everything about the BFF's server-side session: the
// Store interface tokens are persisted behind, the long-lived session
// cookie, and the short-lived pre-auth transaction cookie used to carry
// PKCE/state/nonce across the Auth0 redirect round trip.
package session

import (
	"context"
	"time"
)

// Data is what a Store persists per session. The session cookie itself only
// ever holds the opaque ID used to look this up — never any of these
// fields — per the BFF pattern's requirement that tokens stay server-side.
type Data struct {
	Subject      string // Auth0 "sub" claim
	Email        string
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time // access token expiry
}

// Store persists session data keyed by opaque session ID. Phase 2 uses an
// in-memory implementation (internal/store/memory); Phase 3 replaces it with
// a Redis-backed one (internal/store/redis) without touching callers, since
// everything here goes through this interface.
type Store interface {
	Save(ctx context.Context, id string, data Data, ttl time.Duration) error
	Get(ctx context.Context, id string) (Data, error)
	Delete(ctx context.Context, id string) error
}
