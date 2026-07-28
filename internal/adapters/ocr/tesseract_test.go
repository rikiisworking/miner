package ocr_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/ocr"
	"github.com/rikiisworking/miner/internal/ocrtest"
)

func TestTesseract_EmptyImage(t *testing.T) {
	eng, err := newTestTesseract(t)
	if err != nil {
		t.Skip(err.Error())
	}
	_, err = eng.Recognize(nil)
	if !errors.Is(err, ocr.ErrEmptyImage) {
		t.Fatalf("got %v want ErrEmptyImage", err)
	}
}

func TestTesseract_NotFound(t *testing.T) {
	_, err := ocr.NewTesseract(ocr.TesseractConfig{Bin: filepath.Join(t.TempDir(), "no-such-tesseract")})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ocr.ErrTesseractNotFound) {
		t.Fatalf("got %v want ErrTesseractNotFound", err)
	}
}

func TestTesseract_SmokeSingleSentence(t *testing.T) {
	eng, err := newTestTesseract(t)
	if err != nil {
		t.Skip(err.Error())
	}
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	img, err := m.Must("01_single_sentence").Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text, err := eng.Recognize(img)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	// Collapse spaces for compare — engine also normalizes.
	got := strings.ReplaceAll(text, " ", "")
	if !strings.Contains(got, "私は本を読む") {
		t.Fatalf("got %q want contain 私は本を読む", text)
	}
}

func TestTesseract_SmokeMultiSentence(t *testing.T) {
	eng, err := newTestTesseract(t)
	if err != nil {
		t.Skip(err.Error())
	}
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	img, err := m.Must("02_multi_sentence").Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text, err := eng.Recognize(img)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	got := strings.ReplaceAll(text, " ", "")
	if !strings.Contains(got, "病院に行った") || !strings.Contains(got, "私は本を読む") {
		t.Fatalf("multi got %q", text)
	}
}

func TestTesseract_NotAnImageFailsOrEmpty(t *testing.T) {
	eng, err := newTestTesseract(t)
	if err != nil {
		t.Skip(err.Error())
	}
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	img, err := m.Must("19_not_an_image").Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text, err := eng.Recognize(img)
	// Engine may error (preferred) or return empty/garbage; product IngestPage
	// treats empty as ErrEmptyPage and errors as ErrOcrFailed.
	if err != nil {
		if !errors.Is(err, ocr.ErrRecognizeFailed) {
			t.Fatalf("unexpected err type: %v", err)
		}
		return
	}
	if strings.TrimSpace(text) != "" {
		// Some builds return noise; still not a hard fail for adapter smoke.
		t.Logf("not_an_image returned text=%q (tolerated)", text)
	}
}

func TestTesseractContract_HappyPathOverlap(t *testing.T) {
	if os.Getenv("MINER_OCR_CONTRACT") != "1" {
		t.Skip("set MINER_OCR_CONTRACT=1 to run real-engine contract suite")
	}
	eng, err := newTestTesseract(t)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Keep contract small + reliable: horizontal happy fixtures.
	ids := []string{"01_single_sentence", "02_multi_sentence", "05_punctuation_mix", "10_large_sparse"}
	for _, id := range ids {
		c := m.Must(id)
		if !c.WantSuccess || c.ExpectedText == "" {
			continue
		}
		img, err := c.Bytes()
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		got, err := eng.Recognize(img)
		if err != nil {
			t.Errorf("%s: Recognize: %v", id, err)
			continue
		}
		min := 0.7
		if c.MinOverlap != nil {
			min = *c.MinOverlap
		}
		score := runeOverlap(got, c.ExpectedText)
		if score < min {
			t.Errorf("%s: overlap=%.2f < %.2f\n got=%q\nwant=%q", id, score, min, got, c.ExpectedText)
		}
	}
}

// newTestTesseract prefers env (MINER_TESSERACT / PATH). Returns error if unavailable.
func newTestTesseract(t *testing.T) (*ocr.Tesseract, error) {
	t.Helper()
	return ocr.NewTesseract(ocr.TesseractConfig{
		Bin:            os.Getenv("MINER_TESSERACT"),
		Lang:           envOr("MINER_OCR_LANG", ocr.DefaultTesseractLang),
		TessdataPrefix: os.Getenv("MINER_TESSDATA_PREFIX"),
	})
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// runeOverlap is |intersection of non-space runes| / |want non-space runes|.
func runeOverlap(got, want string) float64 {
	g := countRunes(got)
	w := countRunes(want)
	if len(w) == 0 {
		if len(g) == 0 {
			return 1
		}
		return 0
	}
	// multiset intersection
	inter := 0
	for r, wn := range w {
		gn := g[r]
		if gn < wn {
			inter += gn
		} else {
			inter += wn
		}
	}
	total := 0
	for _, n := range w {
		total += n
	}
	return float64(inter) / float64(total)
}

func countRunes(s string) map[rune]int {
	m := map[rune]int{}
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\u3000' {
			continue
		}
		m[r]++
	}
	return m
}

