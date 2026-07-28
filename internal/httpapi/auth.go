package httpapi

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/rikiisworking/miner/internal/app"
)

func (s *Server) handleIndex(c *fiber.Ctx) error {
	ok, err := s.isAuthenticated(c)
	if err != nil {
		return err
	}
	if ok {
		return s.render(c, "shell", nil)
	}
	return s.render(c, "pin", map[string]any{"Error": ""})
}

func (s *Server) handleUnlock(c *fiber.Ctx) error {
	ip := c.IP()
	if !s.unlockAllowed(ip) {
		c.Status(fiber.StatusTooManyRequests)
		return s.render(c, "pin", map[string]any{
			"Error": "Too many PIN attempts. Wait a minute and try again.",
		})
	}

	pin := c.FormValue("pin")
	if err := s.mining.Unlock(pin); err != nil {
		if errors.Is(err, app.ErrInvalidPIN) {
			s.recordUnlockFail(ip)
			c.Status(fiber.StatusUnauthorized)
			return s.render(c, "pin", map[string]any{
				"Error": "Incorrect PIN. Try again.",
			})
		}
		return err
	}
	s.clearUnlockFails(ip)

	sess, err := s.store.Get(c)
	if err != nil {
		return err
	}
	sess.Set(sessionKeyAuth, true)
	if err := sess.Save(); err != nil {
		return err
	}

	return s.render(c, "shell", nil)
}

func (s *Server) handleHome(c *fiber.Ctx) error {
	return s.render(c, "shell", nil)
}

func (s *Server) requireAuth(c *fiber.Ctx) error {
	ok, err := s.isAuthenticated(c)
	if err != nil {
		return err
	}
	if !ok {
		// HTMX: generic error fragment only — never a feature partial (analyze/queue/…).
		if c.Get("HX-Request") == "true" {
			c.Status(fiber.StatusUnauthorized)
			return s.renderHTMXError(c, fiber.StatusUnauthorized, "Session required. Enter PIN.")
		}
		accept := c.Get("Accept")
		if accept == "" || strings.Contains(accept, "text/html") {
			c.Status(fiber.StatusUnauthorized)
			return s.render(c, "pin", map[string]any{
				"Error": "Session required. Enter PIN.",
			})
		}
		return c.SendStatus(fiber.StatusUnauthorized)
	}
	return c.Next()
}

func (s *Server) isAuthenticated(c *fiber.Ctx) (bool, error) {
	sess, err := s.store.Get(c)
	if err != nil {
		return false, err
	}
	v := sess.Get(sessionKeyAuth)
	b, _ := v.(bool)
	return b, nil
}
