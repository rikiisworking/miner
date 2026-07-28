package app

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/rikiisworking/miner/internal/ports"
)

// SentenceAnalysis is the result of AnalyzeSentence: full token stream for furigana
// plus the filtered content-word list for the vocab rows.
type SentenceAnalysis struct {
	Sentence     string
	Tokens       []ports.Token
	ContentWords []ports.Token
	// PassID is an ephemeral id for this analyze result. First AddUnknown with this
	// pass creates the queue entry; later adds with the same pass append. Not durable.
	PassID string
}

// AnalyzeSentence runs the JapaneseAnalyzer and applies the content-word filter.
// Furigana uses Tokens; the list under the sentence uses ContentWords only.
// Analyze alone never writes the queue.
func (m *MiningApp) AnalyzeSentence(text string) (SentenceAnalysis, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return SentenceAnalysis{}, ErrEmptySentence
	}

	tokens, err := m.analyzer.Analyze(text)
	if err != nil {
		return SentenceAnalysis{}, fmt.Errorf("%w: %w", ErrAnalyze, err)
	}
	if tokens == nil {
		tokens = []ports.Token{}
	}

	passID, err := m.newID()
	if err != nil {
		return SentenceAnalysis{}, err
	}

	return SentenceAnalysis{
		Sentence:     text,
		Tokens:       tokens,
		ContentWords: filterContentWords(tokens),
		PassID:       passID,
	}, nil
}

// filterContentWords keeps content tokens that are worth mining as unknowns.
// Rules:
//   - Token.Content must be true (analyzer / test fake)
//   - non-empty surface
//   - surface must contain at least one kanji (Han script)
//
// Pure kana (hiragana/katakana) content tokens are omitted from the vocab list.
// Furigana still uses the full Tokens stream.
func filterContentWords(tokens []ports.Token) []ports.Token {
	out := make([]ports.Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Content && t.Surface != "" && containsKanji(t.Surface) {
			out = append(out, t)
		}
	}
	return out
}

// containsKanji reports whether s has at least one Han ideograph.
func containsKanji(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
