package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// newID returns a random hex id for queue entries and ephemeral pass IDs.
func (m *MiningApp) newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
