package ports

import "context"

// OcrEngine turns image bytes into plain text (local engines only).
// Product: no cloud OCR. Callers own discarding image bytes after Recognize returns.
//
// Output is engine text (stdout hygiene only). Product page-text rules
// (inter-CJK space strip, blank-line collapse) live in MiningApp, not here.
// Tests inject fakes; production wires a local adapter.
type OcrEngine interface {
	// Recognize extracts text from a full image payload (PNG/JPEG/…).
	// Honors ctx cancel/deadline (e.g. CommandContext). Empty image or
	// undecodable bytes may return an error (adapter-defined).
	// Does not write the durable queue.
	Recognize(ctx context.Context, image []byte) (text string, err error)
}
