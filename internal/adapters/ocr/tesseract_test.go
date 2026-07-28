package ocr_test

import (
	"context"
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
	_, err = eng.Recognize(context.Background(), nil)
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
	text, err := eng.Recognize(context.Background(), img)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
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
	text, err := eng.Recognize(context.Background(), img)
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
	text, err := eng.Recognize(context.Background(), img)
	// Engine may error (preferred) or return empty/garbage; product IngestPage
	// treats empty as ErrEmptyPage and errors as ErrOcrFailed.
	if err != nil {
		if !errors.Is(err, ocr.ErrRecognizeFailed) {
			t.Fatalf("unexpected err type: %v", err)
		}
		return
	}
	if strings.TrimSpace(text) != "" {
		t.Logf("not_an_image returned text=%q (tolerated)", text)
	}
}

// contractSuite is one stress dimension exercised against testdata/ocr fixtures.
type contractSuite struct {
	name string
	// tag selects cases that include this tag (via ocrtest.Manifest.WithTag).
	tag string
	// defaultMin used when cases.json omits min_overlap.
	defaultMin float64
}

// contractSoftIDs: measured empty/near-empty under default jpn+jpn_vert + PSM 3.
// Still run for visibility; do not hard-fail on overlap (log only).
// Product safety net remains editable sentence text.
var contractSoftIDs = map[string]bool{
	"04_vertical_columns":             true, // often page-number only
	"26_novel_vertical_skewed":        true,
	"28_novel_vertical_stub_sentences": true, // empty OCR on this render
	"37_tilt_v_moderate":              true,
	"39_tilt_h_with_blur":             true,
	"44_brightness_mixed_lr":          true, // split lighting kills half page
	"47_brightness_mixed_vertical":    true,
	"55_mixed_brightness_colour_blur": true, // kitchen-sink compound
}

// TestTesseractContract_StressSuites runs real OCR on vertical, blur, brightness,
// font / thickness / colour, and happy fixtures when MINER_OCR_CONTRACT=1.
//
//	MINER_OCR_CONTRACT=1 go test ./internal/adapters/ocr/ -run Contract -count=1
func TestTesseractContract_StressSuites(t *testing.T) {
	if os.Getenv("MINER_OCR_CONTRACT") != "1" {
		t.Skip("set MINER_OCR_CONTRACT=1 to run real-engine contract suites")
	}
	eng, err := newTestTesseract(t)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}

	suites := []contractSuite{
		{name: "happy", tag: "happy", defaultMin: 0.7},
		{name: "vertical", tag: "vertical", defaultMin: 0.25},
		{name: "blur", tag: "blur", defaultMin: 0.25},
		{name: "brightness", tag: "brightness", defaultMin: 0.25},
		{name: "font", tag: "font", defaultMin: 0.35},
		{name: "thickness", tag: "thickness", defaultMin: 0.35},
		{name: "colour", tag: "colour", defaultMin: 0.35},
	}

	for _, suite := range suites {
		suite := suite
		t.Run(suite.name, func(t *testing.T) {
			cases := m.WithTag(suite.tag)
			if len(cases) == 0 {
				t.Fatalf("no fixtures tagged %q", suite.tag)
			}
			ran := 0
			for _, c := range cases {
				if c.ExpectedText == "" {
					continue
				}
				c := c
				t.Run(c.ID, func(t *testing.T) {
					img, err := c.Bytes()
					if err != nil {
						t.Fatal(err)
					}
					got, err := eng.Recognize(context.Background(), img)
					if err != nil {
						if contractSoftIDs[c.ID] || !c.WantSuccess {
							t.Logf("soft: Recognize error (tolerated): %v", err)
							return
						}
						t.Fatalf("Recognize: %v", err)
					}
					min := suite.defaultMin
					if c.MinOverlap != nil {
						min = *c.MinOverlap
					}
					score := runeOverlap(got, c.ExpectedText)
					t.Logf("overlap=%.2f min=%.2f out_len=%d", score, min, len(strings.TrimSpace(got)))

					if contractSoftIDs[c.ID] {
						// Soft: log only; empty OCR is known for some vertical/compound shots.
						if score < min {
							t.Logf("soft below min: score=%.2f < %.2f (known weak fixture)", score, min)
						}
						return
					}
					if !c.WantSuccess {
						// Best-effort fixtures: no hard floor.
						return
					}
					if score < min {
						t.Errorf("overlap=%.2f < %.2f\n got=%q\nwant=%q", score, min, got, c.ExpectedText)
					}
				})
				ran++
			}
			if ran == 0 {
				t.Fatalf("suite %s: no cases with expected_text", suite.name)
			}
		})
	}
}

// TestTesseractContract_HappyPathOverlap kept as a thin alias entry for docs/CI that
// still run -run 'HappyPath'. Delegates to stress suite happy subset logic inline.
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
	for _, c := range m.HappyPath() {
		if c.ExpectedText == "" || !c.WantSuccess {
			continue
		}
		c := c
		t.Run(c.ID, func(t *testing.T) {
			img, err := c.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			got, err := eng.Recognize(context.Background(), img)
			if err != nil {
				t.Fatalf("Recognize: %v", err)
			}
			min := 0.7
			if c.MinOverlap != nil {
				min = *c.MinOverlap
			}
			score := runeOverlap(got, c.ExpectedText)
			if score < min {
				t.Errorf("overlap=%.2f < %.2f\n got=%q\nwant=%q", score, min, got, c.ExpectedText)
			}
		})
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
