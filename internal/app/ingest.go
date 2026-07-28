package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/rikiisworking/miner/internal/ports"
)

// SentenceRegion is one clickable sentence overlay on a captured page image.
// X/Y/W/H are normalized 0–1 relative to image width/height (display-scale safe).
type SentenceRegion struct {
	Text string
	X, Y float64
	W, H float64
}

// PageIngest is the result of IngestPage: product-normalized OCR text plus candidates.
// Ephemeral only — never written to the durable queue.
// On success both Text and Candidates are non-empty.
// Regions may be empty when the engine provides no line geometry.
type PageIngest struct {
	// Text is OCR output after NormalizePageText.
	Text string
	// Candidates are sentence proposals for pick → analyze.
	Candidates []string
	// Regions are clickable sentence boxes (normalized). Best-effort; may be empty.
	Regions []SentenceRegion
	// ImgW/ImgH are source image pixels from the engine (0 if unknown).
	ImgW, ImgH int
}

// IngestPage runs local OCR on image bytes and proposes sentence candidates.
//
// Rules:
//   - len(image) == 0 → ErrEmptyImage
//   - len(image) > MaxUploadBytes → ErrPayloadTooLarge (OCR not run)
//   - only one ingest at a time (ErrIngestBusy if concurrent)
//   - ctx cancel/deadline during OCR → ErrIngestCanceled; busy slot released
//   - OCR error → ErrOcrFailed (wrapped); queue untouched
//   - empty / whitespace after NormalizePageText → ErrEmptyPage; queue untouched
//   - success → NormalizePageText then SplitSentences (+ optional regions); no queue write
//
// ctx nil is treated as context.Background(). Product MaxIngestDuration bounds wait.
// Callers must discard image bytes after return (success or failure).
func (m *MiningApp) IngestPage(ctx context.Context, image []byte) (PageIngest, error) {
	if len(image) == 0 {
		return PageIngest{}, ErrEmptyImage
	}
	if len(image) > MaxUploadBytes {
		return PageIngest{}, ErrPayloadTooLarge
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.ingestMu.Lock()
	if m.ingesting {
		m.ingestMu.Unlock()
		return PageIngest{}, ErrIngestBusy
	}
	m.ingesting = true
	m.ingestMu.Unlock()

	defer func() {
		m.ingestMu.Lock()
		m.ingesting = false
		m.ingestMu.Unlock()
	}()

	runCtx, cancel := context.WithTimeout(ctx, MaxIngestDuration)
	defer cancel()

	raw, err := m.ocr.Recognize(runCtx, image)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return PageIngest{}, fmt.Errorf("%w: %w", ErrIngestCanceled, err)
		}
		return PageIngest{}, fmt.Errorf("%w: %w", ErrOcrFailed, err)
	}
	text, cands, err := candidatesFromPageText(raw.Text)
	if err != nil {
		return PageIngest{}, err
	}
	regions := MapLinesToSentenceRegions(raw.Lines, cands, raw.Width, raw.Height)
	return PageIngest{
		Text:       text,
		Candidates: cands,
		Regions:    regions,
		ImgW:       raw.Width,
		ImgH:       raw.Height,
	}, nil
}

// ProposeSentences is the MiningApp facade for page-text segmentation (legacy paste API).
// Applies NormalizePageText then SplitSentences (same pipeline tail as IngestPage).
// Does not write the durable queue or open analyze passes.
// Empty / whitespace-only page text returns ErrEmptyPage so HTTP stays a pure mapper.
func (m *MiningApp) ProposeSentences(pageText string) ([]string, error) {
	_, cands, err := candidatesFromPageText(pageText)
	return cands, err
}

// candidatesFromPageText is the shared normalize → split tail for OCR and paste.
func candidatesFromPageText(raw string) (text string, cands []string, err error) {
	text = NormalizePageText(raw)
	if text == "" {
		return "", nil, ErrEmptyPage
	}
	cands = SplitSentences(text)
	if len(cands) == 0 {
		return "", nil, ErrEmptyPage
	}
	return text, cands, nil
}

// MapLinesToSentenceRegions maps OCR line boxes onto SplitSentences candidates.
//
// Strategy: walk lines in order, consume into the current sentence by matching
// compact (whitespace-stripped) text; union pixel rects of contributing lines;
// normalize to 0–1 using imgW/imgH.
//
// Returns nil when geometry cannot be produced (no lines, zero size, or match fail).
func MapLinesToSentenceRegions(lines []ports.OcrLine, sentences []string, imgW, imgH int) []SentenceRegion {
	if len(sentences) == 0 || len(lines) == 0 || imgW <= 0 || imgH <= 0 {
		return nil
	}

	type rect struct{ x0, y0, x1, y1 int }
	union := func(a *rect, x, y, w, h int) {
		if w <= 0 || h <= 0 {
			return
		}
		x1, y1 := x+w, y+h
		if a.x1 == 0 && a.y1 == 0 && a.x0 == 0 && a.y0 == 0 {
			// distinguish unset: use first real rect
			*a = rect{x, y, x1, y1}
			return
		}
		if x < a.x0 {
			a.x0 = x
		}
		if y < a.y0 {
			a.y0 = y
		}
		if x1 > a.x1 {
			a.x1 = x1
		}
		if y1 > a.y1 {
			a.y1 = y1
		}
	}
	valid := func(a rect) bool {
		return a.x1 > a.x0 && a.y1 > a.y0
	}

	// Compact removes whitespace so line breaks / OCR spaces do not break match.
	compact := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if unicode.IsSpace(r) {
				continue
			}
			b.WriteRune(r)
		}
		return b.String()
	}

	// Build full compact stream from lines for sequential consume.
	type lineSpan struct {
		start, end int // in compact stream
		line       ports.OcrLine
	}
	var stream strings.Builder
	var spans []lineSpan
	for _, ln := range lines {
		c := compact(ln.Text)
		if c == "" {
			continue
		}
		start := stream.Len()
		stream.WriteString(c)
		spans = append(spans, lineSpan{start: start, end: stream.Len(), line: ln})
	}
	full := stream.String()
	if full == "" {
		return nil
	}

	out := make([]SentenceRegion, 0, len(sentences))
	pos := 0
	for _, sent := range sentences {
		cs := compact(sent)
		if cs == "" {
			continue
		}
		// Forward-only match (fail closed on OCR/normalize drift — no rewind).
		idx := strings.Index(full[pos:], cs)
		if idx < 0 {
			continue
		}
		idx += pos
		end := idx + len(cs)

		var r rect
		set := false
		for _, sp := range spans {
			// Overlap with [idx, end)
			if sp.end <= idx || sp.start >= end {
				continue
			}
			union(&r, sp.line.X, sp.line.Y, sp.line.W, sp.line.H)
			set = true
		}
		pos = end
		if !set || !valid(r) {
			continue
		}
		out = append(out, SentenceRegion{
			Text: sent,
			X:    float64(r.x0) / float64(imgW),
			Y:    float64(r.y0) / float64(imgH),
			W:    float64(r.x1-r.x0) / float64(imgW),
			H:    float64(r.y1-r.y0) / float64(imgH),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
