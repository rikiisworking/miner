package ports

// Token is one analyzer output unit for a span of the sentence.
// Surface and Reading drive furigana; Content drives the content-word list.
type Token struct {
	// Surface is the form as it appears in the sentence (not forced to dictionary form).
	Surface string
	// Reading is kana reading for furigana; may be empty when unknown or unnecessary.
	Reading string
	// Content is true for tokens that belong in the content-word list.
	// Product baseline: keep nouns, verbs, adjectives, adjectival nouns (and similar);
	// drop particles, auxiliary verbs, symbols, punctuation, pure function words.
	// Real adapters map engine POS tags into this flag; tests set it explicitly.
	Content bool
}

// JapaneseAnalyzer turns sentence text into tokens (surface, reading, content flag).
// Local engines only in product; tests inject fakes.
type JapaneseAnalyzer interface {
	// Analyze tokenizes text. Returns a non-nil error when analysis cannot complete.
	Analyze(text string) ([]Token, error)
}
