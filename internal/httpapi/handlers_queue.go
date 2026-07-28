package httpapi

import "github.com/gofiber/fiber/v2"

func (s *Server) handleQueue(c *fiber.Ctx) error {
	entries, err := s.mining.ListQueue()
	if err != nil {
		return s.renderUnexpected(c, err)
	}
	return s.render(c, "queue", map[string]any{
		"Entries": entries,
	})
}

func (s *Server) handleExport(c *fiber.Ctx) error {
	md, err := s.mining.ExportMarkdown()
	if err != nil {
		return s.renderUnexpected(c, err)
	}
	c.Set("Content-Type", "text/markdown; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="miner-export.md"`)
	return c.SendString(md)
}

func (s *Server) handleClearQueue(c *fiber.Ctx) error {
	if err := s.mining.ClearAll(); err != nil {
		return s.renderUnexpected(c, err)
	}
	// Queue UI uses plain form POST (no hx-post); always redirect to empty page.
	return c.Redirect("/queue", fiber.StatusSeeOther)
}
