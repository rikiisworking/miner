package httpapi

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"

	"github.com/rikiisworking/miner/internal/app"
)

const (
	sessionKeyAuth = "authenticated"
	cookieName     = "miner_session"
)

// Server is the Fiber HTTP adapter over MiningApp.
type Server struct {
	app       *app.MiningApp
	fiber     *fiber.App
	store     *session.Store
	templates *template.Template
	addr      string
}

// Config wires the HTTP adapter.
type Config struct {
	MiningApp *app.MiningApp
	// WebFS root must contain templates/*.html and static/ (e.g. htmx.min.js).
	WebFS fs.FS
	Addr  string
}

// New builds a Fiber app with PIN gate routes.
func New(cfg Config) (*Server, error) {
	if cfg.MiningApp == nil {
		return nil, errors.New("MiningApp is required")
	}
	if cfg.WebFS == nil {
		return nil, errors.New("WebFS is required")
	}
	addr := cfg.Addr
	if addr == "" {
		addr = ":8080"
	}

	tmpl, err := template.ParseFS(cfg.WebFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	mem := memory.New()
	sess := session.New(session.Config{
		Storage: mem,
		// Cookie is session-scoped; server-side session lives until process restart (in-memory store).
		KeyLookup:      "cookie:" + cookieName,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   false,
		CookiePath:     "/",
	})

	s := &Server{
		app:       cfg.MiningApp,
		store:     sess,
		templates: tmpl,
		addr:      addr,
	}

	f := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	staticFS, err := fs.Sub(cfg.WebFS, "static")
	if err != nil {
		return nil, fmt.Errorf("static subfs: %w", err)
	}
	f.Use("/static", filesystem.New(filesystem.Config{
		Root:   http.FS(staticFS),
		Browse: false,
	}))

	f.Get("/", s.handleIndex)
	f.Post("/unlock", s.handleUnlock)
	f.Get("/home", s.requireAuth, s.handleHome)

	s.fiber = f
	return s, nil
}

// App exposes the underlying Fiber app (tests).
func (s *Server) App() *fiber.App { return s.fiber }

// Addr returns the listen address.
func (s *Server) Addr() string { return s.addr }

// Listen starts the HTTP server (blocking).
func (s *Server) Listen() error {
	return s.fiber.Listen(s.addr)
}

// Shutdown stops the server.
func (s *Server) Shutdown() error {
	return s.fiber.Shutdown()
}

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
	pin := c.FormValue("pin")
	if err := s.app.Unlock(pin); err != nil {
		if errors.Is(err, app.ErrInvalidPIN) {
			c.Status(fiber.StatusUnauthorized)
			return s.render(c, "pin", map[string]any{
				"Error": "Incorrect PIN. Try again.",
			})
		}
		return err
	}

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
