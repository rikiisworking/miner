// Package ocr provides OcrEngine adapters.
package ocr

import (
	"errors"

	"github.com/rikiisworking/miner/internal/ports"
)

// ErrNotConfigured is returned by Stub when no fixture/text is configured.
var ErrNotConfigured = errors.New("ocr: not configured")

// Stub is a deterministic OcrEngine for wiring and tests until a real local engine lands.
// Default Recognize fails; set Text or ByBytes for controlled success.
type Stub struct {
	// Text, when non-empty, is returned for every Recognize call (ignores image).
	Text string
	// ByBytes maps exact image payloads to text. Checked before Text.
	ByBytes map[string]string
	// FailWith, when set, makes every Recognize call fail with this error.
	FailWith error
}

// Recognize implements ports.OcrEngine.
func (s Stub) Recognize(image []byte) (string, error) {
	if s.FailWith != nil {
		return "", s.FailWith
	}
	if s.ByBytes != nil {
		if t, ok := s.ByBytes[string(image)]; ok {
			return t, nil
		}
	}
	if s.Text != "" {
		return s.Text, nil
	}
	return "", ErrNotConfigured
}

// Ensure Stub satisfies the port at compile time.
var _ ports.OcrEngine = Stub{}
