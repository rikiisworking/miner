package app_test

import (
	"strings"
	"testing"

	"github.com/rikiisworking/miner/internal/app"
)

func TestNormalizePageText_VerticalSpacedGlyphs(t *testing.T) {
	in := "彼 は 夜 の 街 を 歩 い た 。\n\n雨 が 静 か に 降 っ て い た 。\n"
	got := app.NormalizePageText(in)
	want := "彼は夜の街を歩いた。\n雨が静かに降っていた。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizePageText_KeepsSpaceBetweenASCIIWords(t *testing.T) {
	in := "Hello world 私 は 本"
	got := app.NormalizePageText(in)
	if !strings.Contains(got, "Hello world") {
		t.Fatalf("ASCII spaces must remain: %q", got)
	}
	if strings.Contains(got, "私 は") {
		t.Fatalf("CJK spaces should strip: %q", got)
	}
}

func TestNormalizePageText_Empty(t *testing.T) {
	if got := app.NormalizePageText("  \n\t  "); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizePageText_CRLF_AndIdeographicSpace(t *testing.T) {
	in := "私\u3000は\r\n本\u3000を\r読む。"
	got := app.NormalizePageText(in)
	if strings.Contains(got, "\r") {
		t.Fatalf("CR leftover: %q", got)
	}
	if strings.Contains(got, "\u3000") || strings.Contains(got, "私 は") {
		t.Fatalf("ideographic/space not stripped: %q", got)
	}
	if !strings.Contains(got, "私は") || !strings.Contains(got, "本を") {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizePageText_BlankLinesCollapsed(t *testing.T) {
	in := "病院に行った。\n\n\n\n私は本を読む。"
	got := app.NormalizePageText(in)
	if strings.Contains(got, "\n\n") {
		t.Fatalf("blank lines not collapsed: %q", got)
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("lines=%v", lines)
	}
}
