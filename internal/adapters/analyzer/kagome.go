package analyzer

import (
	"errors"
	"strings"
	"unicode"

	"github.com/ikawaha/kagome-dict/ipa"
	"github.com/ikawaha/kagome/v2/tokenizer"

	"github.com/rikiisworking/miner/internal/ports"
)

// Kagome is the production JapaneseAnalyzer: pure-Go kagome + MeCab-IPADIC.
// Maps engine POS tags into Token.Content using the package baseline (see package doc).
type Kagome struct {
	tok *tokenizer.Tokenizer
}

// NewKagome builds a tokenizer with the embedded IPA dictionary.
// Safe to share across goroutines after construction (Tokenize is concurrent-safe).
func NewKagome() (*Kagome, error) {
	tok, err := tokenizer.New(ipa.Dict(), tokenizer.OmitBosEos())
	if err != nil {
		return nil, err
	}
	return &Kagome{tok: tok}, nil
}

// Analyze implements ports.JapaneseAnalyzer.
func (k *Kagome) Analyze(text string) ([]ports.Token, error) {
	if k == nil || k.tok == nil {
		return nil, errors.New("kagome analyzer: not initialized")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("kagome analyzer: empty text")
	}

	raw := k.tok.Tokenize(text)
	out := make([]ports.Token, 0, len(raw))
	for _, t := range raw {
		surface := t.Surface
		if surface == "" {
			continue
		}
		pos := t.POS()
		content := isContentPOS(pos)
		reading := ""
		if content {
			reading = readingForFurigana(surface, t)
		}
		out = append(out, ports.Token{
			Surface: surface,
			Reading: reading,
			Content: content,
		})
	}
	return out, nil
}

// isContentPOS applies the product baseline against MeCab-IPADIC major POS.
//
//	Keep: 名詞 (incl. 形容動詞語幹), 動詞, 形容詞, 副詞, 連体詞, 感動詞, 接頭詞, 接尾辞
//	Drop: 助詞, 助動詞, 記号, 接続詞, フィラー, その他, empty/unknown majors
func isContentPOS(pos []string) bool {
	if len(pos) == 0 {
		return false
	}
	switch pos[0] {
	case "名詞", "動詞", "形容詞", "副詞", "連体詞", "感動詞", "接頭詞", "接尾辞":
		return true
	default:
		return false
	}
}

// readingForFurigana returns hiragana reading for content tokens when useful.
// Empty when engine has no reading, or surface is already pure kana (no ruby needed).
func readingForFurigana(surface string, t tokenizer.Token) string {
	r, ok := t.Reading()
	if !ok || r == "" || r == "*" {
		return ""
	}
	hira := katakanaToHiragana(r)
	if hira == "" {
		return ""
	}
	// Pure-kana surface: ruby is noise (e.g. already-hiragana loan stems).
	if isAllKana(surface) {
		return ""
	}
	return hira
}

func katakanaToHiragana(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'ァ' && r <= 'ン':
			b.WriteRune(r - 0x60)
		case r == 'ヴ':
			b.WriteRune('ゔ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isAllKana(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
			continue
		}
		// Prolonged sound mark and iteration marks still "kana-ish".
		if r == 'ー' || r == 'ゝ' || r == 'ゞ' || r == 'ヽ' || r == 'ヾ' {
			continue
		}
		return false
	}
	return true
}

var _ ports.JapaneseAnalyzer = (*Kagome)(nil)
