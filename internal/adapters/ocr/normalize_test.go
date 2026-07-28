package ocr

import (
	"strings"
	"testing"
)

func TestNormalizeOCRText_VerticalSpacedGlyphs(t *testing.T) {
	in := "彼 は 夜 の 街 を 歩 い た 。\n\n雨 が 静 か に 降 っ て い た 。\n"
	got := normalizeOCRText(in)
	want := "彼は夜の街を歩いた。\n雨が静かに降っていた。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeOCRText_KeepsSpaceBetweenASCIIWords(t *testing.T) {
	in := "Hello world 私 は 本"
	got := normalizeOCRText(in)
	if !strings.Contains(got, "Hello world") {
		t.Fatalf("ASCII spaces must remain: %q", got)
	}
	if strings.Contains(got, "私 は") {
		t.Fatalf("CJK spaces should strip: %q", got)
	}
}

func TestImageExt(t *testing.T) {
	if imageExt([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) != ".png" {
		t.Fatal("png")
	}
	if imageExt([]byte{0xff, 0xd8, 0xff, 0xe0}) != ".jpg" {
		t.Fatal("jpg")
	}
	if imageExt([]byte{1, 2, 3}) != ".img" {
		t.Fatal("unknown")
	}
}
