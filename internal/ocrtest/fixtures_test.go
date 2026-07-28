package ocrtest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rikiisworking/miner/internal/ocrtest"
)

func TestManifestHasAtLeastTenCasesWithFiles(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Cases) < 10 {
		t.Fatalf("want >= 10 OCR cases, got %d", len(m.Cases))
	}
	if m.MaxUploadBytes != ocrtest.MaxUploadBytes {
		t.Fatalf("manifest max_upload_bytes=%d, package const=%d", m.MaxUploadBytes, ocrtest.MaxUploadBytes)
	}

	seen := map[string]bool{}
	for _, c := range m.Cases {
		if c.ID == "" {
			t.Fatal("case with empty id")
		}
		if seen[c.ID] {
			t.Fatalf("duplicate case id %q", c.ID)
		}
		seen[c.ID] = true

		if c.File == "" {
			t.Fatalf("%s: empty file", c.ID)
		}
		p := c.Path()
		if !filepath.IsAbs(p) {
			t.Fatalf("%s: path not absolute: %s", c.ID, p)
		}
		b, err := c.Bytes()
		if err != nil {
			t.Fatalf("%s: read %s: %v", c.ID, p, err)
		}
		if len(b) == 0 {
			t.Fatalf("%s: empty file %s", c.ID, p)
		}
		// Product rule: fixtures themselves stay under upload cap (oversize is in-test buffer).
		if len(b) > ocrtest.MaxUploadBytes {
			t.Fatalf("%s: fixture %d bytes exceeds MaxUploadBytes; generate oversize in test code", c.ID, len(b))
		}
	}
}

func TestHappyPathCasesHaveExpectedText(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	happy := m.HappyPath()
	if len(happy) < 3 {
		t.Fatalf("want several happy cases, got %d", len(happy))
	}
	for _, c := range happy {
		if c.ExpectedText == "" {
			t.Errorf("%s: happy case missing expected_text", c.ID)
		}
		if !c.WantSuccess {
			t.Errorf("%s: happy case should want_success", c.ID)
		}
	}
}

func TestErrorPathNotAnImage(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := m.Must("19_not_an_image")
	if c.WantSuccess {
		t.Fatal("19_not_an_image should want_success=false")
	}
	b, err := c.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	// Must not look like PNG/JPEG magic — forces decode/OCR error paths.
	if len(b) >= 8 && string(b[:8]) == "\x89PNG\r\n\x1a\n" {
		t.Fatal("19_not_an_image unexpectedly PNG")
	}
	if len(b) >= 2 && b[0] == 0xff && b[1] == 0xd8 {
		t.Fatal("19_not_an_image unexpectedly JPEG")
	}
}

func TestStubAlignedSingleSentence(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := m.Must("01_single_sentence")
	const want = "私は本を読む。"
	if c.ExpectedText != want {
		t.Fatalf("expected_text=%q want %q", c.ExpectedText, want)
	}
	if !c.HasTag("stub-aligned") {
		t.Fatal("want stub-aligned tag for analyzer fixture bridge")
	}
}

func TestTinySmokeFixture(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := m.Must("15_tiny")
	b, err := c.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 8*1024 {
		t.Fatalf("15_tiny should stay tiny for L2, got %d bytes", len(b))
	}
	if !c.HasTag("L2-smoke") {
		t.Fatal("want L2-smoke tag")
	}
}

func TestMultiSentenceFeedsSplit(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := m.Must("02_multi_sentence")
	if c.ExpectedText == "" {
		t.Fatal("empty expected")
	}
	// Rough: at least two Japanese sentence terminators for splitSentences.
	n := 0
	for _, r := range c.ExpectedText {
		if r == '。' || r == '！' || r == '？' {
			n++
		}
	}
	if n < 2 {
		t.Fatalf("02_multi_sentence want >=2 terminators, got %d in %q", n, c.ExpectedText)
	}
}

func TestNovelVerticalTateGakiSuite(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Full bunkobon-style suite: tag "novel" + "tate-gaki" (plus legacy 04).
	tate := m.WithTag("tate-gaki")
	if len(tate) < 8 {
		t.Fatalf("want >=8 tate-gaki novel cases, got %d", len(tate))
	}
	required := []string{
		"21_novel_vertical_page",
		"22_novel_vertical_dialogue",
		"23_novel_vertical_dense",
		"24_novel_vertical_chapter_open",
		"25_novel_vertical_single_col",
		"26_novel_vertical_skewed",
		"27_novel_vertical_punctuation",
		"28_novel_vertical_stub_sentences",
	}
	for _, id := range required {
		c := m.Must(id)
		if !c.HasTag("vertical") || !c.HasTag("novel") {
			t.Errorf("%s: want tags vertical+novel", id)
		}
		if c.ExpectedText == "" {
			t.Errorf("%s: empty expected_text", id)
		}
		b, err := c.Bytes()
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		if len(b) < 1000 {
			t.Errorf("%s: image too small (%d B) for novel page fixture", id, len(b))
		}
	}
	// Dialogue must carry Japanese quotes for OCR stress.
	d := m.Must("22_novel_vertical_dialogue")
	if !strings.Contains(d.ExpectedText, "「") || !strings.Contains(d.ExpectedText, "」") {
		t.Fatal("22 dialogue expected_text missing 「」")
	}
	// Stub-aligned vertical bridges known analyzer sentences.
	s := m.Must("28_novel_vertical_stub_sentences")
	if !s.HasTag("stub-aligned") {
		t.Fatal("28 want stub-aligned")
	}
	if !strings.Contains(s.ExpectedText, "私は本を読む。") {
		t.Fatalf("28 missing stub sentence: %q", s.ExpectedText)
	}
}

func TestSlightTiltPhoneShotSuite(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	tilted := m.WithTag("tilt")
	if len(tilted) < 8 {
		t.Fatalf("want >=8 tilt cases, got %d", len(tilted))
	}
	// Dedicated slight-tilt suite (31–40) plus older skew fixtures tagged tilt.
	required := []string{
		"31_tilt_h_slight_cw",
		"32_tilt_h_slight_ccw",
		"33_tilt_h_moderate",
		"34_tilt_h_strong",
		"35_tilt_v_slight_cw",
		"36_tilt_v_slight_ccw",
		"37_tilt_v_moderate",
		"38_tilt_h_multi_sentence",
		"39_tilt_h_with_blur",
		"40_tilt_v_dialogue",
	}
	slight := 0
	for _, id := range required {
		c := m.Must(id)
		if !c.HasTag("tilt") || !c.HasTag("phone-capture") {
			t.Errorf("%s: want tags tilt+phone-capture", id)
		}
		if c.ExpectedText == "" {
			t.Errorf("%s: empty expected_text", id)
		}
		if _, err := c.Bytes(); err != nil {
			t.Errorf("%s: %v", id, err)
		}
		if c.HasTag("slight") {
			slight++
		}
	}
	if slight < 4 {
		t.Fatalf("want several 'slight' tilt cases, got %d", slight)
	}
	// Both lean directions for horizontal slight.
	if !m.Must("31_tilt_h_slight_cw").HasTag("horizontal") {
		t.Fatal("31 want horizontal")
	}
	if !m.Must("32_tilt_h_slight_ccw").HasTag("horizontal") {
		t.Fatal("32 want horizontal")
	}
	// Vertical slight pair.
	for _, id := range []string{"35_tilt_v_slight_cw", "36_tilt_v_slight_ccw"} {
		c := m.Must(id)
		if !c.HasTag("tate-gaki") || !c.HasTag("vertical") {
			t.Errorf("%s: want tate-gaki+vertical", id)
		}
	}
}

func TestBrightnessAndMixedStyleSuite(t *testing.T) {
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	// Global + mixed brightness.
	brightnessIDs := []string{
		"41_brightness_dim",
		"42_brightness_bright",
		"43_brightness_very_dark",
		"44_brightness_mixed_lr",
		"45_brightness_mixed_tb",
		"46_brightness_gradient",
		"47_brightness_mixed_vertical",
	}
	for _, id := range brightnessIDs {
		c := m.Must(id)
		if !c.HasTag("brightness") {
			t.Errorf("%s: want brightness tag", id)
		}
		if c.ExpectedText == "" {
			t.Errorf("%s: empty expected_text", id)
		}
		if _, err := c.Bytes(); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
	mixedBright := 0
	for _, c := range m.WithTag("brightness") {
		if c.HasTag("mixed") {
			mixedBright++
		}
	}
	if mixedBright < 3 {
		t.Fatalf("want several mixed-brightness cases, got %d", mixedBright)
	}

	// Intra-shot style / partial blur mix.
	styleIDs := []string{
		"48_partial_blur_bottom",
		"49_partial_blur_center_band",
		"50_mixed_fonts",
		"51_mixed_thickness",
		"52_mixed_colours",
		"53_mixed_font_thickness_colour",
		"54_mixed_style_partial_blur",
		"55_mixed_brightness_colour_blur",
	}
	for _, id := range styleIDs {
		c := m.Must(id)
		if !c.HasTag("mixed") && !c.HasTag("partial") {
			t.Errorf("%s: want mixed or partial tag", id)
		}
		if c.ExpectedText == "" {
			t.Errorf("%s: empty expected_text", id)
		}
		if _, err := c.Bytes(); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
	if !m.Must("50_mixed_fonts").HasTag("font") {
		t.Fatal("50 want font tag")
	}
	if !m.Must("51_mixed_thickness").HasTag("thickness") {
		t.Fatal("51 want thickness tag")
	}
	if !m.Must("52_mixed_colours").HasTag("colour") {
		t.Fatal("52 want colour tag")
	}
	if !m.Must("48_partial_blur_bottom").HasTag("blur") {
		t.Fatal("48 want blur tag")
	}
	// Compound kitchen-sink case.
	ks := m.Must("55_mixed_brightness_colour_blur")
	for _, tag := range []string{"brightness", "colour", "blur", "compound"} {
		if !ks.HasTag(tag) {
			t.Errorf("55 missing tag %s", tag)
		}
	}
}
