package app

import "strings"

// Sentence terminators for page-text segmentation (ticket 05).
// Baseline: Japanese 。！？ plus fullwidth ． and halfwidth ! ? for mixed paste.
// Split keeps the terminator on the preceding candidate.
var sentenceTerminators = map[rune]bool{
	'。': true, // ideographic full stop
	'！': true, // fullwidth exclamation
	'？': true, // fullwidth question
	'．': true, // fullwidth full stop
	'!':  true, // halfwidth
	'?':  true, // halfwidth
}

// SplitSentences segments page text into candidate sentences.
//
// Rules (product baseline; edit remains the safety net):
//   - empty / whitespace-only → empty slice (documented)
//   - split after 。！？ and fullwidth/halfwidth variants above; terminator stays with sentence
//   - trailing text without terminator becomes its own candidate
//   - if no terminator appears, whole trimmed text is one editable candidate
//   - consecutive terminators do not yield empty candidates
//
// Pure helper: no I/O, no queue, not OcrEngine.
func SplitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	var out []string
	var b strings.Builder
	for _, r := range text {
		b.WriteRune(r)
		if sentenceTerminators[r] {
			s := strings.TrimSpace(b.String())
			if s != "" {
				out = append(out, s)
			}
			b.Reset()
		}
	}
	rest := strings.TrimSpace(b.String())
	if rest != "" {
		out = append(out, rest)
	}
	if len(out) == 0 {
		// Defensive: non-empty input but nothing emitted → one blob.
		return []string{text}
	}
	return out
}

// ProposeSentences is the MiningApp facade for page-text segmentation.
// Does not write the durable queue or open analyze passes.
func (m *MiningApp) ProposeSentences(pageText string) []string {
	return SplitSentences(pageText)
}
