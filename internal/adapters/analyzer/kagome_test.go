package analyzer_test

import (
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
	"github.com/rikiisworking/miner/internal/ports"
)

func mustKagome(t *testing.T) *analyzer.Kagome {
	t.Helper()
	k, err := analyzer.NewKagome()
	if err != nil {
		t.Fatalf("NewKagome: %v", err)
	}
	return k
}

func TestKagome_DemoSentenceBook(t *testing.T) {
	toks, err := mustKagome(t).Analyze("私は本を読む。")
	if err != nil {
		t.Fatal(err)
	}
	// Same morphs as historical stub fixtures: 私 は 本 を 読む 。
	wantSurfaces := []string{"私", "は", "本", "を", "読む", "。"}
	if len(toks) != len(wantSurfaces) {
		t.Fatalf("len=%d want %d: %+v", len(toks), len(wantSurfaces), toks)
	}
	for i, s := range wantSurfaces {
		if toks[i].Surface != s {
			t.Fatalf("tok[%d].Surface=%q want %q", i, toks[i].Surface, s)
		}
	}
	assertContent(t, toks, map[string]bool{
		"私": true, "は": false, "本": true, "を": false, "読む": true, "。": false,
	})
	assertReading(t, toks, "私", "わたし")
	assertReading(t, toks, "本", "ほん")
	assertReading(t, toks, "読む", "よむ")
	assertReading(t, toks, "は", "")
	assertReading(t, toks, "を", "")
	assertReading(t, toks, "。", "")
}

func TestKagome_DemoSentenceHospital(t *testing.T) {
	toks, err := mustKagome(t).Analyze("病院に行った。")
	if err != nil {
		t.Fatal(err)
	}
	wantSurfaces := []string{"病院", "に", "行っ", "た", "。"}
	if len(toks) != len(wantSurfaces) {
		t.Fatalf("len=%d want %d: %+v", len(toks), len(wantSurfaces), toks)
	}
	for i, s := range wantSurfaces {
		if toks[i].Surface != s {
			t.Fatalf("tok[%d].Surface=%q want %q", i, toks[i].Surface, s)
		}
	}
	assertContent(t, toks, map[string]bool{
		"病院": true, "に": false, "行っ": true, "た": false, "。": false,
	})
	assertReading(t, toks, "病院", "びょういん")
	assertReading(t, toks, "行っ", "いっ")
	assertReading(t, toks, "た", "")
}

func TestKagome_Empty(t *testing.T) {
	_, err := mustKagome(t).Analyze("   ")
	if err == nil {
		t.Fatal("expected empty-text error")
	}
}

func TestKagome_ArbitraryTextSplits(t *testing.T) {
	// Stub fallback was one blob; real engine must split.
	toks, err := mustKagome(t).Analyze("雨が降る")
	if err != nil {
		t.Fatal(err)
	}
	if len(toks) < 3 {
		t.Fatalf("want multi-token split, got %+v", toks)
	}
	var content []string
	for _, tok := range toks {
		if tok.Content {
			content = append(content, tok.Surface)
		}
	}
	if !containsAll(content, "雨", "降る") {
		t.Fatalf("content words missing 雨/降る: %v", content)
	}
}

func TestKagome_NaAdjectiveStemIsContent(t *testing.T) {
	toks, err := mustKagome(t).Analyze("静かな夜")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tok := range toks {
		if tok.Surface == "静か" && tok.Content {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("want 静か as content (形容動詞語幹 under 名詞): %+v", toks)
	}
}

func TestKagome_ParticleAndAuxDroppedFromContent(t *testing.T) {
	toks, err := mustKagome(t).Analyze("食べた")
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range toks {
		if tok.Surface == "た" && tok.Content {
			t.Fatalf("auxiliary た must be non-content: %+v", toks)
		}
	}
}

func assertContent(t *testing.T, toks []ports.Token, want map[string]bool) {
	t.Helper()
	for _, tok := range toks {
		w, ok := want[tok.Surface]
		if !ok {
			continue
		}
		if tok.Content != w {
			t.Fatalf("surface %q Content=%v want %v", tok.Surface, tok.Content, w)
		}
	}
}

func assertReading(t *testing.T, toks []ports.Token, surface, want string) {
	t.Helper()
	for _, tok := range toks {
		if tok.Surface == surface {
			if tok.Reading != want {
				t.Fatalf("surface %q Reading=%q want %q", surface, tok.Reading, want)
			}
			return
		}
	}
	t.Fatalf("surface %q not found in %+v", surface, toks)
}

func containsAll(have []string, want ...string) bool {
	set := make(map[string]bool, len(have))
	for _, h := range have {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}
