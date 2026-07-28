package analyzer_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
	"github.com/rikiisworking/miner/internal/ports"
)

var (
	sharedKagome     *analyzer.Kagome
	sharedKagomeOnce sync.Once
	sharedKagomeErr  error
)

func mustKagome(t *testing.T) *analyzer.Kagome {
	t.Helper()
	sharedKagomeOnce.Do(func() {
		sharedKagome, sharedKagomeErr = analyzer.NewKagome()
	})
	if sharedKagomeErr != nil {
		t.Fatalf("NewKagome: %v", sharedKagomeErr)
	}
	return sharedKagome
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

func TestKagome_Analyze_NotInitialized(t *testing.T) {
	var nilK *analyzer.Kagome
	if _, err := nilK.Analyze("私は本を読む。"); err == nil {
		t.Fatal("nil receiver: expected error, no panic")
	}
	zero := &analyzer.Kagome{}
	if _, err := zero.Analyze("私は本を読む。"); err == nil {
		t.Fatal("zero Kagome: expected error, no panic")
	}
}

func TestKagome_PureKanaSurface_EmptyReading(t *testing.T) {
	// Pure-kana content surfaces must not carry furigana (ruby is noise).
	toks, err := mustKagome(t).Analyze("する")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tok := range toks {
		if tok.Surface == "する" {
			found = true
			if !tok.Content {
				t.Fatalf("する should be content: %+v", toks)
			}
			if tok.Reading != "" {
				t.Fatalf("pure-kana surface Reading=%q want empty", tok.Reading)
			}
		}
		if tok.Content && tok.Reading != "" {
			// Any pure-kana content token in this input must also be empty.
			allKana := true
			for _, r := range tok.Surface {
				if r < 'ぁ' || (r > 'ん' && r < 'ァ') || r > 'ン' {
					if r != 'ー' && r != 'ゔ' && r != 'ヴ' {
						allKana = false
						break
					}
				}
			}
			if allKana && tok.Reading != "" {
				t.Fatalf("pure-kana content %q Reading=%q want empty", tok.Surface, tok.Reading)
			}
		}
	}
	if !found {
		t.Fatalf("surface する not found: %+v", toks)
	}
}

func TestKagome_MatchesStubFixtureSurfaces_DemoSentences(t *testing.T) {
	k := mustKagome(t)
	stub := analyzer.Stub{}
	for _, text := range []string{"私は本を読む。", "病院に行った。"} {
		want, err := stub.Analyze(text)
		if err != nil {
			t.Fatalf("stub %q: %v", text, err)
		}
		got, err := k.Analyze(text)
		if err != nil {
			t.Fatalf("kagome %q: %v", text, err)
		}
		if len(got) != len(want) {
			t.Fatalf("%q: len got=%d want=%d\n got=%+v\nwant=%+v", text, len(got), len(want), got, want)
		}
		for i := range want {
			if got[i].Surface != want[i].Surface {
				t.Fatalf("%q tok[%d].Surface got=%q want=%q", text, i, got[i].Surface, want[i].Surface)
			}
		}
	}
}

func TestKagome_ConcurrentAnalyze(t *testing.T) {
	k := mustKagome(t)
	const n = 32
	var wg sync.WaitGroup
	errCh := make(chan error, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			toks, err := k.Analyze("私は本を読む。")
			if err != nil {
				errCh <- err
				return
			}
			if len(toks) != 6 {
				errCh <- fmt.Errorf("unexpected token count %d", len(toks))
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
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
