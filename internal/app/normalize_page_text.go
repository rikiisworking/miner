package app

import (
	"strings"
	"unicode"
)

// NormalizePageText prepares OCR or paste text for sentence segmentation.
//
// Product rules (owned by MiningApp, not the OcrEngine adapter):
//   - normalize newlines
//   - strip spaces that sit between Japanese glyphs (vertical OCR often inserts them)
//   - drop blank lines
//   - trim
//
// Pure helper: no I/O. Call before SplitSentences on engine output.
func NormalizePageText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = stripInterCJKSpaces(s)

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// stripInterCJKSpaces drops ASCII/ideographic spaces between Japanese characters
// (engines often emit "私 は 本" or per-glyph spaces on vertical text).
func stripInterCJKSpaces(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		if isSpaceRune(r) && i > 0 && i+1 < len(runes) {
			prev, next := runes[i-1], runes[i+1]
			if isJPGlyph(prev) && isJPGlyph(next) {
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\u3000'
}

// isJPGlyph: hiragana, katakana, CJK, common punctuation used in novel prose.
func isJPGlyph(r rune) bool {
	if unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) {
		return true
	}
	switch r {
	case '。', '、', '！', '？', '．', '「', '」', '『', '』', '（', '）',
		'・', 'ー', '—', '…', '々', '〆', '〇', '〜', '～',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'０', '１', '２', '３', '４', '５', '６', '７', '８', '９':
		return true
	}
	return false
}
