// Package app provides MiningApp, the application facade for product use-cases.
//
// Architectural rule: product rules live here (and behind ports this package owns).
// HTTP (Fiber) adapters must stay thin — map transport to MiningApp, do not re-implement
// analyze/queue/export/auth decisions in handlers. L1 tests exercise this package.
package app

import (
	"errors"

	"github.com/rikiisworking/miner/internal/ports"
)

// ErrInvalidPIN is returned when Unlock is called with a wrong PIN.
var ErrInvalidPIN = errors.New("invalid pin")

// MiningApp is the application facade for product use-cases.
// HTTP adapters stay thin over this type.
type MiningApp struct {
	pinAuth ports.PinAuth
}

// NewMiningApp constructs the facade with required ports.
func NewMiningApp(pinAuth ports.PinAuth) *MiningApp {
	if pinAuth == nil {
		panic("pinAuth is required")
	}
	return &MiningApp{pinAuth: pinAuth}
}

// Unlock verifies the shared PIN. On success the HTTP layer may establish a session.
// Domain layer does not own cookies or HTTP sessions.
func (m *MiningApp) Unlock(pin string) error {
	if !m.pinAuth.Verify(pin) {
		return ErrInvalidPIN
	}
	return nil
}
