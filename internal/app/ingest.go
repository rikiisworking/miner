package app

import (
	"context"
	"errors"
	"fmt"
)

// PageIngest is the result of IngestPage: product-normalized OCR text plus candidates.
// Ephemeral only — never written to the durable queue.
// On success both Text and Candidates are non-empty.
type PageIngest struct {
	// Text is OCR output after NormalizePageText.
	Text string
	// Candidates are sentence proposals for the existing pick → analyze pipeline.
	Candidates []string
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
//   - success → NormalizePageText then SplitSentences; no queue / analyze pass write
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

	text, err := m.ocr.Recognize(runCtx, image)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return PageIngest{}, fmt.Errorf("%w: %w", ErrIngestCanceled, err)
		}
		return PageIngest{}, fmt.Errorf("%w: %w", ErrOcrFailed, err)
	}
	text = NormalizePageText(text)
	if text == "" {
		return PageIngest{}, ErrEmptyPage
	}
	return PageIngest{Text: text, Candidates: SplitSentences(text)}, nil
}

// ProposeSentences is the MiningApp facade for page-text segmentation (paste path).
// Applies NormalizePageText then SplitSentences (same pipeline tail as IngestPage).
// Does not write the durable queue or open analyze passes.
// Empty / whitespace-only page text returns ErrEmptyPage so HTTP stays a pure mapper.
func (m *MiningApp) ProposeSentences(pageText string) ([]string, error) {
	cands := SplitSentences(NormalizePageText(pageText))
	if len(cands) == 0 {
		return nil, ErrEmptyPage
	}
	return cands, nil
}
