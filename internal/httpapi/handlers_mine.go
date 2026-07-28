package httpapi

import (
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"

	"github.com/rikiisworking/miner/internal/app"
)

// handlePageText proposes sentence candidates from multi-sentence page paste (ticket 05).
// Ephemeral only — does not write the durable queue.
func (s *Server) handlePageText(c *fiber.Ctx) error {
	pageText := c.FormValue("page_text")
	cands, err := s.mining.ProposeSentences(pageText)
	if err != nil {
		if errors.Is(err, app.ErrEmptyPage) {
			return s.renderCandidatesErr(c, fiber.StatusBadRequest, "Enter page text to split into sentences.")
		}
		return s.renderUnexpected(c, err)
	}
	return s.render(c, "sentence_candidates", map[string]any{
		"Error":      "",
		"Candidates": cands,
	})
}

// handleIngest runs photo OCR via MiningApp.IngestPage (ticket 06).
// Multipart field "image". Image bytes are not saved to disk; discarded after return.
// Reuses sentence_candidates partial (same pick → analyze pipeline as page-text).
func (s *Server) handleIngest(c *fiber.Ctx) error {
	fh, err := c.FormFile("image")
	if err != nil || fh == nil {
		return s.renderCandidatesErr(c, fiber.StatusBadRequest, "Choose an image of a novel page.")
	}
	// Header size is a cheap pre-check; MiningApp still enforces MaxUploadBytes on bytes.
	if fh.Size > app.MaxUploadBytes {
		return s.renderCandidatesErr(c, fiber.StatusRequestEntityTooLarge, "Image too large (max 10 MB).")
	}

	f, err := fh.Open()
	if err != nil {
		return s.renderCandidatesErr(c, fiber.StatusBadRequest, "Could not read the uploaded image.")
	}
	defer f.Close()

	// Cap read so a lying Content-Length cannot blow memory past product + 1.
	image, err := io.ReadAll(io.LimitReader(f, int64(app.MaxUploadBytes)+1))
	if err != nil {
		return s.renderCandidatesErr(c, fiber.StatusBadRequest, "Could not read the uploaded image.")
	}

	ingested, err := s.mining.IngestPage(c.UserContext(), image)
	if err != nil {
		return s.renderIngestError(c, err)
	}
	return s.render(c, "sentence_candidates", map[string]any{
		"Error":      "",
		"Candidates": ingested.Candidates,
	})
}

func (s *Server) handleAnalyze(c *fiber.Ctx) error {
	sentence := c.FormValue("sentence")
	result, err := s.mining.AnalyzeSentence(sentence)
	if err != nil {
		msg := "Analysis failed. Try again."
		status := fiber.StatusUnprocessableEntity
		if errors.Is(err, app.ErrEmptySentence) {
			msg = "Enter a sentence to analyze."
			status = fiber.StatusBadRequest
		} else if errors.Is(err, app.ErrAnalyze) {
			msg = "Analysis failed. The sentence could not be tokenized."
		} else {
			return s.renderUnexpected(c, err)
		}
		c.Status(status)
		return s.render(c, "analyze_result", map[string]any{
			"Error": msg,
		})
	}

	return s.render(c, "analyze_result", map[string]any{
		"Error":        "",
		"Sentence":     result.Sentence,
		"Tokens":       result.Tokens,
		"ContentWords": result.ContentWords,
		"PassID":       result.PassID,
		"EntryID":      "", // filled after first unknown
	})
}

func (s *Server) handleAddUnknown(c *fiber.Ctx) error {
	sentence := c.FormValue("sentence")
	surface := c.FormValue("surface")
	entryID := c.FormValue("entry_id")
	passID := c.FormValue("pass_id")

	res, err := s.mining.AddUnknown(sentence, surface, entryID, passID)
	if err != nil {
		msg := "Could not save unknown."
		status := fiber.StatusInternalServerError
		if errors.Is(err, app.ErrEmptySurface) {
			msg = "Missing word surface."
			status = fiber.StatusBadRequest
		} else if errors.Is(err, app.ErrEmptySentence) {
			msg = "Missing sentence."
			status = fiber.StatusBadRequest
		} else if errors.Is(err, app.ErrEntryNotFound) {
			msg = "Queue entry not found. Analyze again."
			status = fiber.StatusNotFound
		} else {
			return s.renderUnexpected(c, err)
		}
		c.Status(status)
		return s.render(c, "unknown_feedback", map[string]any{
			"Error": msg,
		})
	}

	// Template only uses Error / EntryID / Surface / Duplicate.
	return s.render(c, "unknown_feedback", map[string]any{
		"Error":     "",
		"EntryID":   res.EntryID,
		"Surface":   res.Surface,
		"Duplicate": res.Duplicate,
	})
}
