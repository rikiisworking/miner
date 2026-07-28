package ocr

import "testing"

// MustEngine returns a production NDLOCR-Lite OcrEngine for tests that
// intentionally exercise the real pipeline (OCR smoke, fixture quality, live-glyph ingest).
// Skips the test with install hints if the worker / models are not available —
// product tests that do not need OCR should use Static instead so they never skip.
func MustEngine(t testing.TB) *NDL {
	t.Helper()
	eng, err := NewNDLFromEnv()
	if err != nil {
		t.Skipf("NDLOCR-Lite required: %v\nInstall ndlocr-lite + deps, then set:\n  MINER_NDL_ROOT=/path/to/ndlocr-lite\n  MINER_NDL_PYTHON=/path/to/venv/bin/python\n  MINER_NDL_WORKER=/path/to/miner/scripts/ndl_ocr_worker.py\nSee requirements-ocr.txt.", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}
