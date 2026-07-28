package ports

import "context"

// OcrLine is one recognized text line with its axis-aligned box in image pixels.
// Coordinates are top-left origin; W/H are positive extents when known.
type OcrLine struct {
	Text string
	X, Y int
	W, H int
}

// OcrResult is local OCR output. Product page-text rules (NormalizePageText,
// SplitSentences, sentence region mapping) live in MiningApp, not here.
//
// Lines may be empty when the engine only returns plain text (e.g. test Static).
// Width/Height are source image pixels; 0 means unknown.
type OcrResult struct {
	Text   string
	Lines  []OcrLine
	Width  int
	Height int
}

// OcrEngine turns image bytes into text (and optional line geometry).
// Product: no cloud OCR. Callers own discarding image bytes after Recognize returns.
//
// Tests inject fakes; production wires a local adapter.
type OcrEngine interface {
	// Recognize extracts text from a full image payload (PNG/JPEG/…).
	// Honors ctx cancel/deadline (e.g. CommandContext). Empty image or
	// undecodable bytes may return an error (adapter-defined).
	// Does not write the durable queue.
	Recognize(ctx context.Context, image []byte) (OcrResult, error)
}
