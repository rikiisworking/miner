package httpapi

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/rikiisworking/miner/internal/app"
)

// User-facing oversize copy (header precheck + product ErrPayloadTooLarge).
const msgImageTooLarge = "Image too large (max 10 MB). Use a smaller photo."

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

func (s *Server) renderIngestError(c *fiber.Ctx, err error) error {
	// Phone-readable: short, actionable, mention paste fallback for novel mining.
	msg := "Could not read text from the image. Retake (fill the frame, reduce tilt) or paste page text."
	status := fiber.StatusUnprocessableEntity
	switch {
	case errors.Is(err, app.ErrPayloadTooLarge):
		msg = msgImageTooLarge
		status = fiber.StatusRequestEntityTooLarge
	case errors.Is(err, app.ErrIngestBusy):
		msg = "Already processing a photo. Wait, then try again."
		status = fiber.StatusConflict
	case errors.Is(err, app.ErrIngestCanceled):
		msg = "Photo processing was canceled. Try again."
		status = fiber.StatusRequestTimeout
	case errors.Is(err, app.ErrEmptyImage):
		msg = "Choose an image of a novel page."
		status = fiber.StatusBadRequest
	case errors.Is(err, app.ErrEmptyPage):
		msg = "No text found in the image. Retake a clearer page photo or paste page text."
		status = fiber.StatusBadRequest
	case errors.Is(err, app.ErrOcrFailed):
		// same default msg
	}
	return s.renderCandidatesErr(c, status, msg)
}

// renderUnexpected maps store/I/O failures to an HTMX-friendly fragment when possible.
func (s *Server) renderUnexpected(c *fiber.Ctx, err error) error {
	_ = err // logged by Fiber when ErrorHandler used; avoid leaking internals in HTML
	if c.Get("HX-Request") == "true" {
		return s.renderHTMXError(c, fiber.StatusInternalServerError, "Something went wrong. Try again.")
	}
	return fmt.Errorf("unexpected: %w", err)
}

func (s *Server) renderHTMXError(c *fiber.Ctx, status int, msg string) error {
	c.Status(status)
	return s.render(c, "htmx_error", map[string]any{
		"Error": msg,
	})
}
