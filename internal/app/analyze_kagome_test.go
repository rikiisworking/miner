package app_test

import (
	"sync"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
)

// Shared Kagome instance so the embedded IPA dict loads once for this package's L1 tests.
var (
	kagomeOnce sync.Once
	kagomeInst *analyzer.Kagome
	kagomeErr  error
)

func testKagome(t *testing.T) *analyzer.Kagome {
	t.Helper()
	kagomeOnce.Do(func() {
		kagomeInst, kagomeErr = analyzer.NewKagome()
	})
	if kagomeErr != nil {
		t.Fatalf("NewKagome: %v", kagomeErr)
	}
	return kagomeInst
}

func TestAnalyzeSentence_Kagome_DemoBook_ContentAndPassID(t *testing.T) {
	ja := testKagome(t)
	m := newApp(t, ja)

	sentence := "私は本を読む。"
	got, err := m.AnalyzeSentence(sentence)
	if err != nil {
		t.Fatalf("AnalyzeSentence: %v", err)
	}
	if got.Sentence != sentence {
		t.Fatalf("Sentence=%q want %q", got.Sentence, sentence)
	}
	if got.PassID == "" {
		t.Fatal("expected non-empty PassID")
	}

	// ContentWords: only 私, 本, 読む (particles/punct filtered).
	wantContent := []string{"私", "本", "読む"}
	if len(got.ContentWords) != len(wantContent) {
		t.Fatalf("ContentWords=%+v want surfaces %v", got.ContentWords, wantContent)
	}
	for i, s := range wantContent {
		if got.ContentWords[i].Surface != s {
			t.Fatalf("ContentWords[%d].Surface=%q want %q", i, got.ContentWords[i].Surface, s)
		}
		if !got.ContentWords[i].Content {
			t.Fatalf("ContentWords[%d] not Content: %+v", i, got.ContentWords[i])
		}
	}

	// Full token stream still includes particles for furigana alignment.
	surfaces := make([]string, len(got.Tokens))
	readingBySurface := map[string]string{}
	for i, tok := range got.Tokens {
		surfaces[i] = tok.Surface
		readingBySurface[tok.Surface] = tok.Reading
	}
	if !containsAllSurfaces(surfaces, "は", "を") {
		t.Fatalf("Tokens must include particles は/を: %+v", got.Tokens)
	}
	if readingBySurface["私"] != "わたし" {
		t.Fatalf("reading 私=%q want わたし", readingBySurface["私"])
	}
	if readingBySurface["本"] != "ほん" {
		t.Fatalf("reading 本=%q want ほん", readingBySurface["本"])
	}
	if readingBySurface["読む"] != "よむ" {
		t.Fatalf("reading 読む=%q want よむ", readingBySurface["読む"])
	}

	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("analyze must not write queue; got %d entries", len(list))
	}
}

func TestAnalyzeSentence_Kagome_Hospital_ContentFilter(t *testing.T) {
	ja := testKagome(t)
	m := newApp(t, ja)

	got, err := m.AnalyzeSentence("病院に行った。")
	if err != nil {
		t.Fatalf("AnalyzeSentence: %v", err)
	}
	wantContent := []string{"病院", "行っ"}
	if len(got.ContentWords) != len(wantContent) {
		t.Fatalf("ContentWords=%+v want surfaces %v", got.ContentWords, wantContent)
	}
	for i, s := range wantContent {
		if got.ContentWords[i].Surface != s {
			t.Fatalf("ContentWords[%d].Surface=%q want %q", i, got.ContentWords[i].Surface, s)
		}
	}
	// Particles/aux/punct must not leak into content list.
	for _, w := range got.ContentWords {
		if w.Surface == "に" || w.Surface == "た" || w.Surface == "。" {
			t.Fatalf("particle/function/punct in ContentWords: %+v", w)
		}
	}
}

func TestAddUnknown_AfterKagomeAnalyze_SamePass(t *testing.T) {
	ja := testKagome(t)
	m := newApp(t, ja)

	sentence := "私は本を読む。"
	analysis, err := m.AnalyzeSentence(sentence)
	if err != nil {
		t.Fatalf("AnalyzeSentence: %v", err)
	}
	if analysis.PassID == "" {
		t.Fatal("expected PassID from analyze")
	}

	first, err := m.AddUnknown(sentence, "本", "", analysis.PassID)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Created {
		t.Fatalf("first tap should create: %+v", first)
	}

	second, err := m.AddUnknown(sentence, "読む", "", analysis.PassID)
	if err != nil {
		t.Fatal(err)
	}
	if second.EntryID != first.EntryID {
		t.Fatalf("same pass must share entry: first=%q second=%q", first.EntryID, second.EntryID)
	}
	if second.Created {
		t.Fatal("second tap must not create another entry for the same pass")
	}

	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("queue entries=%d want 1", len(list))
	}
	if len(list[0].Unknowns) != 2 || list[0].Unknowns[0] != "本" || list[0].Unknowns[1] != "読む" {
		t.Fatalf("unknowns=%v want [本 読む]", list[0].Unknowns)
	}
}

func containsAllSurfaces(have []string, want ...string) bool {
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
