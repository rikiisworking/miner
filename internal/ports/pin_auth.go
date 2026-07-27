package ports

// PinAuth verifies the shared PIN secret.
type PinAuth interface {
	// Verify returns true when pin matches the configured secret.
	Verify(pin string) bool
}
