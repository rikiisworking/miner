// Package app provides MiningApp, the application facade for product use-cases.
//
// Architectural rule: product rules live here (and behind ports this package owns).
// HTTP (Fiber) adapters must stay thin — map transport to MiningApp, do not re-implement
// analyze/queue/export/auth decisions in handlers. L1 tests exercise this package.
package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/rikiisworking/miner/internal/ports"
)

// ErrInvalidPIN is returned when Unlock is called with a wrong PIN.
var ErrInvalidPIN = errors.New("invalid pin")

// ErrEmptySentence is returned when AnalyzeSentence receives blank text.
var ErrEmptySentence = errors.New("empty sentence")

// ErrAnalyze is returned when the analyzer port fails (wrapped with cause).
var ErrAnalyze = errors.New("analysis failed")

// MiningApp is the application facade for product use-cases.
// HTTP adapters stay thin over this type.
type MiningApp struct {
	pinAuth  ports.PinAuth
	analyzer ports.JapaneseAnalyzer
}

// NewMiningApp constructs the facade with required ports.
func NewMiningApp(pinAuth ports.PinAuth, analyzer ports.JapaneseAnalyzer) *MiningApp {
	if pinAuth == nil {
		panic("pinAuth is required")
	}
	if analyzer == nil {
		panic("analyzer is required")
	}
	return &MiningApp{pinAuth: pinAuth, analyzer: analyzer}
}

// Unlock verifies the shared PIN. On success the HTTP layer may establish a session.
// Domain layer does not own cookies or HTTP sessions.
func (m *MiningApp) Unlock(pin string) error {
	if !m.pinAuth.Verify(pin) {
		return ErrInvalidPIN
	}
	return nil
}

// SentenceAnalysis is the result of AnalyzeSentence: full token stream for furigana
// plus the filtered content-word list for the vocab rows.
type SentenceAnalysis struct {
	Sentence     string
	Tokens       []ports.Token
	ContentWords []ports.Token
}

// AnalyzeSentence runs the JapaneseAnalyzer and applies the content-word filter.
// Furigana uses Tokens; the list under the sentence uses ContentWords only.
func (m *MiningApp) AnalyzeSentence(text string) (SentenceAnalysis, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return SentenceAnalysis{}, ErrEmptySentence
	}

	tokens, err := m.analyzer.Analyze(text)
	if err != nil {
		return SentenceAnalysis{}, fmt.Errorf("%w: %v", ErrAnalyze, err)
	}
	if tokens == nil {
		tokens = []ports.Token{}
	}

	return SentenceAnalysis{
		Sentence:     text,
		Tokens:       tokens,
		ContentWords: filterContentWords(tokens),
	}, nil
}

// filterContentWords keeps content tokens with a non-empty surface.
// Baseline product rule is encoded on Token.Content by the analyzer (or test fake).
func filterContentWords(tokens []ports.Token) []ports.Token {
	out := make([]ports.Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Content && t.Surface != "" {
			out = append(out, t)
		}
	}
	return out
}
