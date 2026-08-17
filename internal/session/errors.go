package session

import "errors"

var (
	// ErrNoSession is returned when the request carries no session cookie,
	// or the cookie's value is empty.
	ErrNoSession = errors.New("session: no session cookie present")

	// ErrNotFound is returned by a Store when a session ID doesn't resolve
	// to any stored data (expired, revoked, or never existed).
	ErrNotFound = errors.New("session: not found")

	// ErrNoTxn is returned when the request carries no valid auth
	// transaction cookie (missing, expired, or malformed).
	ErrNoTxn = errors.New("session: no auth transaction cookie present")
)
