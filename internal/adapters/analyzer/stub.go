// Package analyzer provides JapaneseAnalyzer adapters.
//
// POS / content mapping (baseline product rule):
//
//	Keep as Content=true:  nouns, verbs, adjectives, adjectival nouns (na-adj), similar content.
//	Drop as Content=false: particles, auxiliary verbs, symbols, punctuation, pure function words.
//
// Production: Kagome (MeCab-IPADIC) via NewKagome — pure Go, no host install.
// Tests/L1–L3 harnesses: Stub (deterministic fixtures + error hooks).
package analyzer

import (
	"errors"
	"strings"

	"github.com/rikiisworking/miner/internal/ports"
)

// ForceErrorText is a special input that makes Stub.Analyze fail (L3 error-path hook).
const ForceErrorText = "__analyze_error__"

// Stub is a deterministic JapaneseAnalyzer with fixture sentences and a safe fallback.
// Optional fields make it the shared L1/L2/L3 test double (map override + error hooks).
type Stub struct {
	// ByText maps exact sentence text to tokens (overrides package fixtures when set).
	ByText map[string][]ports.Token
	// FailWith, when set, makes every Analyze call fail.
	FailWith error
	// FailOn makes Analyze fail only for that exact text.
	FailOn string
}

// Analyze implements ports.JapaneseAnalyzer.
func (s Stub) Analyze(text string) ([]ports.Token, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("stub analyzer: empty text")
	}
	if s.FailWith != nil {
		return nil, s.FailWith
	}
	if s.FailOn != "" && text == s.FailOn {
		return nil, errors.New("forced analyzer failure")
	}
	if text == ForceErrorText {
		return nil, errors.New("stub analyzer: forced failure")
	}
	if s.ByText != nil {
		if toks, ok := s.ByText[text]; ok {
			out := make([]ports.Token, len(toks))
			copy(out, toks)
			return out, nil
		}
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
		{Surface: "私", Reading: "わたし", Content: true}, // noun
		{Surface: "は", Reading: "", Content: false},   // particle
		{Surface: "本", Reading: "ほん", Content: true},  // noun
		{Surface: "を", Reading: "", Content: false},   // particle
		{Surface: "読む", Reading: "よむ", Content: true}, // verb
		{Surface: "。", Reading: "", Content: false},   // punctuation
	},
	"病院に行った。": {
		{Surface: "病院", Reading: "びょういん", Content: true},
		{Surface: "に", Reading: "", Content: false},
		{Surface: "行っ", Reading: "いっ", Content: true},
		{Surface: "た", Reading: "", Content: false},
		{Surface: "。", Reading: "", Content: false},
	},
}
