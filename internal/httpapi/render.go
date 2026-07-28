package httpapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/rikiisworking/miner/internal/app"
)

// User-facing ingest copy (JSON capture + legacy HTML candidates).
const (
	msgImageTooLarge   = "Image too large (max 10 MB). Use a smaller photo."
	msgCaptureNeeded   = "Capture a photo of a novel page."
	msgImageUnreadable = "Could not read the image."
	msgIngestBusy      = "Already processing a photo. Wait, then try again."
	msgIngestCanceled  = "Photo processing was canceled. Try again."
	msgEmptyPage       = "No text found in the image. Retake a clearer page photo."
	msgOcrFailed       = "Could not read text from the image. Retake (fill the frame, reduce tilt)."
	msgSessionRequired = "Session required. Enter PIN."
	msgGenericError    = "Something went wrong. Try again."
)

func wantsJSON(c *fiber.Ctx) bool {
	accept := strings.ToLower(c.Get("Accept"))
	return strings.Contains(accept, "application/json")
}

func (s *Server) render(c *fiber.Ctx, name string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	c.Type("html", "utf-8")
	var b strings.Builder
	if err := s.templates.ExecuteTemplate(&b, name, data); err != nil {
		return err
	}
	return c.SendString(b.String())
}

func (s *Server) renderCandidatesErr(c *fiber.Ctx, status int, msg string) error {
	c.Status(status)
	return s.render(c, "sentence_candidates", map[string]any{
		"Error":      msg,
		"Candidates": nil,
	})
}

// respondIngestErr is the single transport seam for ingest failures (JSON capture UI).
func (s *Server) respondIngestErr(c *fiber.Ctx, status int, msg string) error {
	if wantsJSON(c) {
		return c.Status(status).JSON(fiber.Map{"error": msg})
	}
	return s.renderCandidatesErr(c, status, msg)
}

// respondIngestOK returns capture JSON for successful OCR.
func (s *Server) respondIngestOK(c *fiber.Ctx, ingested app.PageIngest) error {
	regions := make([]fiber.Map, 0, len(ingested.Regions))
	for _, r := range ingested.Regions {
		regions = append(regions, fiber.Map{
			"text": r.Text,
			"x":    r.X,
			"y":    r.Y,
			"w":    r.W,
			"h":    r.H,
		})
	}
	return c.JSON(fiber.Map{
		"candidates": ingested.Candidates,
		"regions":    regions,
		"img_w":      ingested.ImgW,
		"img_h":      ingested.ImgH,
	})
}

func (s *Server) renderIngestError(c *fiber.Ctx, err error) error {
	msg := msgOcrFailed
	status := fiber.StatusUnprocessableEntity
	switch {
	case errors.Is(err, app.ErrPayloadTooLarge):
		msg = msgImageTooLarge
		status = fiber.StatusRequestEntityTooLarge
	case errors.Is(err, app.ErrIngestBusy):
		msg = msgIngestBusy
		status = fiber.StatusConflict
	case errors.Is(err, app.ErrIngestCanceled):
		msg = msgIngestCanceled
		status = fiber.StatusRequestTimeout
	case errors.Is(err, app.ErrEmptyImage):
		msg = msgCaptureNeeded
		status = fiber.StatusBadRequest
	case errors.Is(err, app.ErrEmptyPage):
		msg = msgEmptyPage
		status = fiber.StatusBadRequest
	case errors.Is(err, app.ErrOcrFailed):
		// default msg
	}
	return s.respondIngestErr(c, status, msg)
}

// renderUnexpected maps store/I/O failures to an HTMX-friendly fragment when possible.
func (s *Server) renderUnexpected(c *fiber.Ctx, err error) error {
	_ = err // avoid leaking internals in HTML/JSON
	if c.Get("HX-Request") == "true" {
		return s.renderHTMXError(c, fiber.StatusInternalServerError, msgGenericError)
	}
	if wantsJSON(c) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": msgGenericError})
	}
	return fmt.Errorf("unexpected: %w", err)
}

func (s *Server) renderHTMXError(c *fiber.Ctx, status int, msg string) error {
	c.Status(status)
	return s.render(c, "htmx_error", map[string]any{
		"Error": msg,
	})
}
