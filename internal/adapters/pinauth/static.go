package pinauth

import (
	"crypto/subtle"
)

// Static verifies a PIN against a configured secret using constant-time compare.
type Static struct {
	Secret string
}

// Verify implements ports.PinAuth.
func (s Static) Verify(pin string) bool {
	if s.Secret == "" {
		return false
	}
	// subtle.ConstantTimeCompare requires equal length; pad-compare via HMAC-style length check.
	a := []byte(pin)
	b := []byte(s.Secret)
	if len(a) != len(b) {
		// Still run a dummy compare to reduce trivial timing leak on length.
		subtle.ConstantTimeCompare(b, b)
		return false
	}
	return subtle.ConstantTimeCompare(a, b) == 1
}
