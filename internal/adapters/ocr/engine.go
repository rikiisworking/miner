package ocr

import "testing"

// MustEngine returns a production Tesseract OcrEngine for tests.
// Fatals with install hints if the binary / langs are not available.
// Prefer this over fakes: product rules that need OCR should exercise the real adapter.
func MustEngine(t testing.TB) *Tesseract {
	t.Helper()
	eng, err := NewTesseractFromEnv()
	if err != nil {
		t.Fatalf("tesseract required: %v\nInstall tesseract-ocr + Japanese data (jpn, jpn_vert),\nor set MINER_TESSERACT and MINER_TESSDATA_PREFIX.", err)
	}
	return eng
}
