package app_test

import (
	"errors"
	"testing"

	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/ports"
)

// fakePinAuth is a test double for ports.PinAuth. It does not use production secrets.
type fakePinAuth struct {
	valid string
}

func (f fakePinAuth) Verify(pin string) bool {
	return pin == f.valid
}

// fakeAnalyzer is a test double for ports.JapaneseAnalyzer.
type fakeAnalyzer struct {
	// byText maps exact sentence text to tokens. Explicit Content flags on tokens.
	byText map[string][]ports.Token
	// failWith, when set, makes every Analyze call fail.
	failWith error
	// failOn makes Analyze fail only for that exact text.
	failOn string
}

func (f fakeAnalyzer) Analyze(text string) ([]ports.Token, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	if f.failOn != "" && text == f.failOn {
		return nil, errors.New("forced analyzer failure")
	}
	if f.byText != nil {
		if toks, ok := f.byText[text]; ok {
			return toks, nil
		}
	}
	return []ports.Token{{Surface: text, Reading: "", Content: true}}, nil
}

func newApp(t *testing.T, analyzer ports.JapaneseAnalyzer) *app.MiningApp {
	t.Helper()
	if analyzer == nil {
		analyzer = fakeAnalyzer{}
	}
	return app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"}, analyzer)
}

func TestUnlock_AcceptsCorrectPIN(t *testing.T) {
	m := newApp(t, nil)

	err := m.Unlock("test-pin-ok")
	if err != nil {
		t.Fatalf("Unlock correct PIN: %v", err)
	}
}

func TestUnlock_RejectsWrongPIN(t *testing.T) {
	m := newApp(t, nil)

	err := m.Unlock("wrong")
	if !errors.Is(err, app.ErrInvalidPIN) {
		t.Fatalf("Unlock wrong PIN: got %v, want ErrInvalidPIN", err)
	}
}

func TestUnlock_RejectsEmptyWhenSecretSet(t *testing.T) {
	m := newApp(t, nil)

	err := m.Unlock("")
	if !errors.Is(err, app.ErrInvalidPIN) {
		t.Fatalf("Unlock empty PIN: got %v, want ErrInvalidPIN", err)
	}
}

func TestAnalyzeSentence_ReturnsTokensForFuriganaAndContentList(t *testing.T) {
	sentence := "私は本を読む。"
	tokens := []ports.Token{
		{Surface: "私", Reading: "わたし", Content: true},
		{Surface: "は", Reading: "", Content: false},
		{Surface: "本", Reading: "ほん", Content: true},
		{Surface: "を", Reading: "", Content: false},
		{Surface: "読む", Reading: "よむ", Content: true},
		{Surface: "。", Reading: "", Content: false},
	}
	m := newApp(t, fakeAnalyzer{byText: map[string][]ports.Token{sentence: tokens}})

	got, err := m.AnalyzeSentence(sentence)
	if err != nil {
		t.Fatalf("AnalyzeSentence: %v", err)
	}
	if got.Sentence != sentence {
		t.Fatalf("Sentence=%q want %q", got.Sentence, sentence)
	}
	if len(got.Tokens) != len(tokens) {
		t.Fatalf("Tokens len=%d want %d", len(got.Tokens), len(tokens))
	}
	for i := range tokens {
		if got.Tokens[i] != tokens[i] {
			t.Fatalf("Tokens[%d]=%+v want %+v", i, got.Tokens[i], tokens[i])
		}
	}
	wantContent := []ports.Token{
		{Surface: "私", Reading: "わたし", Content: true},
		{Surface: "本", Reading: "ほん", Content: true},
		{Surface: "読む", Reading: "よむ", Content: true},
	}
	if len(got.ContentWords) != len(wantContent) {
		t.Fatalf("ContentWords=%+v want %+v", got.ContentWords, wantContent)
	}
	for i := range wantContent {
		if got.ContentWords[i] != wantContent[i] {
			t.Fatalf("ContentWords[%d]=%+v want %+v", i, got.ContentWords[i], wantContent[i])
		}
	}
}

func TestAnalyzeSentence_ContentWordFilter_OmitsParticlesAndFunction(t *testing.T) {
	// Stub tokens with explicit content vs non-content flags (product rule under test).
	tokens := []ports.Token{
		{Surface: "病院", Reading: "びょういん", Content: true}, // noun
		{Surface: "に", Reading: "", Content: false},          // particle
		{Surface: "行っ", Reading: "いっ", Content: true},     // verb stem-ish
		{Surface: "た", Reading: "", Content: false},          // auxiliary
		{Surface: "。", Reading: "", Content: false},          // punctuation
	}
	m := newApp(t, fakeAnalyzer{byText: map[string][]ports.Token{"病院に行った。": tokens}})

	got, err := m.AnalyzeSentence("病院に行った。")
	if err != nil {
		t.Fatalf("AnalyzeSentence: %v", err)
	}
	for _, w := range got.ContentWords {
		if !w.Content {
			t.Fatalf("non-content token leaked into ContentWords: %+v", w)
		}
		if w.Surface == "に" || w.Surface == "た" || w.Surface == "。" {
			t.Fatalf("particle/function/punct must be omitted: %+v", w)
		}
	}
	if len(got.ContentWords) != 2 {
		t.Fatalf("ContentWords len=%d want 2 (noun+verb only): %+v", len(got.ContentWords), got.ContentWords)
	}
	// Full token stream still includes particles for furigana alignment.
	if len(got.Tokens) != 5 {
		t.Fatalf("Tokens len=%d want 5 (all stubs for furigana)", len(got.Tokens))
	}
}

func TestAnalyzeSentence_AnalyzerError_IsControlledFailure(t *testing.T) {
	m := newApp(t, fakeAnalyzer{failWith: errors.New("engine down")})

	_, err := m.AnalyzeSentence("何か")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, app.ErrAnalyze) {
		t.Fatalf("got %v, want ErrAnalyze", err)
	}
}

func TestAnalyzeSentence_EmptySentence(t *testing.T) {
	m := newApp(t, fakeAnalyzer{})

	_, err := m.AnalyzeSentence("   ")
	if !errors.Is(err, app.ErrEmptySentence) {
		t.Fatalf("got %v, want ErrEmptySentence", err)
	}
}
