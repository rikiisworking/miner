package ocr

import (
	"context"

	"github.com/rikiisworking/miner/internal/ports"
)

// Static is a fixed-response OcrEngine for tests and harnesses that do not need
// a host NDLOCR-Lite install. Production wires NDL via NewNDLFromEnv.
//
// When Err is non-nil, Recognize returns that error and ignores Text/Lines.
// Otherwise Recognize returns Text (and optional Lines / dimensions) for any
// image payload (including empty), after checking ctx for cancel/deadline.
type Static struct {
	Text   string
	Lines  []ports.OcrLine
	Width  int
	Height int
	Err    error
}

// Recognize implements ports.OcrEngine.
func (s Static) Recognize(ctx context.Context, image []byte) (ports.OcrResult, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return ports.OcrResult{}, err
		}
	}
	if s.Err != nil {
		return ports.OcrResult{}, s.Err
	}
	return ports.OcrResult{
		Text:   s.Text,
		Lines:  s.Lines,
		Width:  s.Width,
		Height: s.Height,
	}, nil
}

var _ ports.OcrEngine = Static{}
