package auth_test

import (
	"testing"

	"github.com/joecris/tdff-bff/internal/auth"
)

func TestNewTransactionIsRandomAndDistinct(t *testing.T) {
	a := auth.NewTransaction()
	b := auth.NewTransaction()

	if a.State == "" || a.Nonce == "" || a.CodeVerifier == "" {
		t.Fatalf("expected non-empty state/nonce/verifier, got %+v", a)
	}
	if a.State == a.Nonce || a.State == a.CodeVerifier || a.Nonce == a.CodeVerifier {
		t.Errorf("expected state/nonce/verifier to be independently random, got %+v", a)
	}
	if a.State == b.State || a.Nonce == b.Nonce || a.CodeVerifier == b.CodeVerifier {
		t.Errorf("expected two transactions to not collide, got a=%+v b=%+v", a, b)
	}
}
