package httpapi

import (
	"errors"
	"io"

	"github.com/gofiber/fiber/v2"

	"github.com/rikiisworking/miner/internal/app"
)

// handlePageText proposes sentence candidates from multi-sentence page paste (legacy API / L2).
// Ephemeral only — not linked in stepped UI.
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

// handleIngest runs photo OCR via MiningApp.IngestPage.
// Multipart field "image". Image bytes are not saved; discarded after return.
// Always returns JSON for the capture UI (candidates + regions).
func (s *Server) handleIngest(c *fiber.Ctx) error {
	// Capture client always sends Accept: application/json; force JSON errors even if omitted.
	if c.Get("Accept") == "" {
		c.Request().Header.Set("Accept", "application/json")
	}

	if s.mining.IngestBusy() {
		return s.renderIngestError(c, app.ErrIngestBusy)
	}

	fh, err := c.FormFile("image")
	if err != nil || fh == nil {
		return s.respondIngestErr(c, fiber.StatusBadRequest, msgCaptureNeeded)
	}
	if fh.Size > app.MaxUploadBytes {
		return s.respondIngestErr(c, fiber.StatusRequestEntityTooLarge, msgImageTooLarge)
	}

	f, err := fh.Open()
	if err != nil {
		return s.respondIngestErr(c, fiber.StatusBadRequest, msgImageUnreadable)
	}
	defer f.Close()

	image, err := io.ReadAll(io.LimitReader(f, int64(app.MaxUploadBytes)+1))
	if err != nil {
		return s.respondIngestErr(c, fiber.StatusBadRequest, msgImageUnreadable)
	}

	ingested, err := s.mining.IngestPage(c.UserContext(), image)
	if err != nil {
		return s.renderIngestError(c, err)
	}
	return s.respondIngestOK(c, ingested)
}

func (s *Server) handleAnalyze(c *fiber.Ctx) error {
	sentence := c.FormValue("sentence")
	result, err := s.mining.AnalyzeSentence(sentence)
	if err != nil {
		status := fiber.StatusUnprocessableEntity
		msg := "Could not analyze this sentence. Go back and pick another."
		if errors.Is(err, app.ErrEmptySentence) {
			msg = "Pick a sentence to analyze."
			status = fiber.StatusBadRequest
		} else if !errors.Is(err, app.ErrAnalyze) {
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
		var (
			status = fiber.StatusBadRequest
			msg    string
		)
		switch {
		case errors.Is(err, app.ErrEmptySurface):
			msg = "Missing word surface."
		case errors.Is(err, app.ErrEmptySentence):
			msg = "Missing sentence."
		case errors.Is(err, app.ErrEntryNotFound):
			msg = "Queue entry not found. Analyze again."
			status = fiber.StatusNotFound
		default:
			return s.renderUnexpected(c, err)
		}
		c.Status(status)
		return s.render(c, "unknown_feedback", map[string]any{
			"Error": msg,
		})
	}

	return s.render(c, "unknown_feedback", map[string]any{
		"Error":     "",
		"EntryID":   res.EntryID,
		"Surface":   res.Surface,
		"Duplicate": res.Duplicate,
	})
}
