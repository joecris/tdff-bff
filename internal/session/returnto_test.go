package session_test

import (
	"testing"

	"github.com/joecris/tdff-bff/internal/session"
)

func TestSanitizeReturnToAcceptsSafePaths(t *testing.T) {
	cases := []string{
		"/",
		"/leagues/123",
		"/leagues/123/picks?tab=results",
		"/a/b/c#section",
	}
	for _, in := range cases {
		if got := session.SanitizeReturnTo(in); got != in {
			t.Errorf("SanitizeReturnTo(%q): expected it unchanged, got %q", in, got)
		}
	}
}

func TestSanitizeReturnToRejectsUnsafeValues(t *testing.T) {
	cases := []string{
		"",                       // empty
		"leagues/123",            // no leading slash
		"//evil.com/phish",       // protocol-relative — different host
		"///evil.com",            // still protocol-relative after normalization
		"http://evil.com/phish",  // absolute URL
		"https://evil.com/phish", // absolute URL, different scheme
		"javascript:alert(1)",    // scheme, no host — url.Parse treats as opaque
		"/\\evil.com",            // backslash some browsers normalize like a slash
		"\\\\evil.com",           // backslash-only variant
		" /leagues/123",          // leading whitespace changes parsing/intent
	}
	for _, in := range cases {
		if got := session.SanitizeReturnTo(in); got != "" {
			t.Errorf("SanitizeReturnTo(%q): expected rejection, got %q", in, got)
		}
	}
}
