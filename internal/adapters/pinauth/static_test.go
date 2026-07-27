package pinauth_test

import (
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/pinauth"
)

func TestStatic_Verify(t *testing.T) {
	a := pinauth.Static{Secret: "secret"}

	if !a.Verify("secret") {
		t.Fatal("expected match")
	}
	if a.Verify("other") {
		t.Fatal("expected mismatch")
	}
	if a.Verify("") {
		t.Fatal("empty should not match")
	}
	if (pinauth.Static{Secret: ""}).Verify("") {
		t.Fatal("empty secret should not accept empty pin")
	}
}
