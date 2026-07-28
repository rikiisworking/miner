package ocr

import "testing"

// MustEngine returns a production Tesseract OcrEngine for tests that intentionally
// exercise the real CLI (OCR smoke, fixture quality, live-glyph ingest).
// Skips the test with install hints if the binary / langs are not available —
// product tests that do not need OCR should use Static instead so they never skip.
func MustEngine(t testing.TB) *Tesseract {
	t.Helper()
	eng, err := NewTesseractFromEnv()
	if err != nil {
		t.Skipf("tesseract required: %v\nInstall tesseract-ocr + Japanese data (jpn, jpn_vert),\nor set MINER_TESSERACT and MINER_TESSDATA_PREFIX.", err)
	}
	return eng
}
