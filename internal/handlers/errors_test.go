package handlers_test

import "errors"

// errBoom is a generic sentinel error for tests that just need "something
// failed" without caring about the specific error value.
var errBoom = errors.New("boom")
