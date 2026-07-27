// Package analyzer provides JapaneseAnalyzer adapters.
//
// POS / content mapping (baseline product rule):
//
//	Keep as Content=true:  nouns, verbs, adjectives, adjectival nouns (na-adj), similar content.
//	Drop as Content=false: particles, auxiliary verbs, symbols, punctuation, pure function words.
//
// This stub does not run a real morphological engine. It serves fixed fixtures for demos/tests
// and a one-token fallback for arbitrary paste text until a real local engine is wired.
package analyzer

import (
	"errors"
	"strings"

	"github.com/rikiisworking/miner/internal/ports"
)

// ForceErrorText is a special input that makes Stub.Analyze fail (L3 error-path hook).
const ForceErrorText = "__analyze_error__"

// Stub is a deterministic JapaneseAnalyzer with fixture sentences and a safe fallback.
type Stub struct{}

// Analyze implements ports.JapaneseAnalyzer.
func (Stub) Analyze(text string) ([]ports.Token, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("stub analyzer: empty text")
	}
	if text == ForceErrorText {
		return nil, errors.New("stub analyzer: forced failure")
	}
	if toks, ok := fixtures[text]; ok {
		// Return a copy so callers cannot mutate the fixture map.
		out := make([]ports.Token, len(toks))
		copy(out, toks)
		return out, nil
	}
	// Fallback: whole string as one content token (no reading). Lets paste path work
	// without a real engine; furigana partial still renders surface-only.
	return []ports.Token{{Surface: text, Reading: "", Content: true}}, nil
}

// fixtures document content vs non-content flags for known demo sentences.
// Real adapters will map engine POS tags into Token.Content using the package baseline.
var fixtures = map[string][]ports.Token{
	"私は本を読む。": {
		{Surface: "私", Reading: "わたし", Content: true},  // noun
		{Surface: "は", Reading: "", Content: false},    // particle
		{Surface: "本", Reading: "ほん", Content: true},   // noun
		{Surface: "を", Reading: "", Content: false},    // particle
		{Surface: "読む", Reading: "よむ", Content: true}, // verb
		{Surface: "。", Reading: "", Content: false},    // punctuation
	},
	"病院に行った。": {
		{Surface: "病院", Reading: "びょういん", Content: true},
		{Surface: "に", Reading: "", Content: false},
		{Surface: "行っ", Reading: "いっ", Content: true},
		{Surface: "た", Reading: "", Content: false},
		{Surface: "。", Reading: "", Content: false},
	},
}
