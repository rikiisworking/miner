package ocr

import (
	"context"

	"github.com/rikiisworking/miner/internal/ports"
)

// Static is a fixed-response OcrEngine for tests and harnesses that do not need
// a host NDLOCR-Lite install. Production wires NDL via NewNDLFromEnv.
//
// When Err is non-nil, Recognize returns that error and ignores Text.
// Otherwise Recognize returns Text for any image payload (including empty),
// after checking ctx for cancel/deadline.
type Static struct {
	Text string
	Err  error
}

// Recognize implements ports.OcrEngine.
func (s Static) Recognize(ctx context.Context, image []byte) (string, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return "", err
		}
	}
	if s.Err != nil {
		return "", s.Err
	}
	return s.Text, nil
}

var _ ports.OcrEngine = Static{}
