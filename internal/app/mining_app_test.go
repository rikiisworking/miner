package app_test

import (
	"errors"
	"testing"

	"github.com/rikiisworking/miner/internal/app"
)

// fakePinAuth is a test double for ports.PinAuth. It does not use production secrets.
type fakePinAuth struct {
	valid string
}

func (f fakePinAuth) Verify(pin string) bool {
	return pin == f.valid
}

func TestUnlock_AcceptsCorrectPIN(t *testing.T) {
	m := app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"})

	err := m.Unlock("test-pin-ok")
	if err != nil {
		t.Fatalf("Unlock correct PIN: %v", err)
	}
}

func TestUnlock_RejectsWrongPIN(t *testing.T) {
	m := app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"})

	err := m.Unlock("wrong")
	if !errors.Is(err, app.ErrInvalidPIN) {
		t.Fatalf("Unlock wrong PIN: got %v, want ErrInvalidPIN", err)
	}
}

func TestUnlock_RejectsEmptyWhenSecretSet(t *testing.T) {
	m := app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"})

	err := m.Unlock("")
	if !errors.Is(err, app.ErrInvalidPIN) {
		t.Fatalf("Unlock empty PIN: got %v, want ErrInvalidPIN", err)
	}
}
