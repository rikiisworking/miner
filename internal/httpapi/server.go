package httpapi

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"

	"github.com/rikiisworking/miner/internal/app"
)

const (
	sessionKeyAuth = "authenticated"
	cookieName     = "miner_session"
	// MultipartOverhead is BodyLimit headroom above MaxUploadBytes for framing.
	MultipartOverhead = 512 * 1024
)

// Server is the Fiber HTTP adapter over MiningApp.
type Server struct {
	mining    *app.MiningApp
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

// New builds a Fiber app with PIN gate and analyze routes.
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
		mining:    cfg.MiningApp,
		store:     sess,
		templates: tmpl,
		addr:      addr,
	}

	// BodyLimit must allow product MaxUploadBytes images plus multipart framing.
	// Semantic oversize still rejected in MiningApp.IngestPage (L1).
	f := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             app.MaxUploadBytes + MultipartOverhead,
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
	f.Post("/page-text", s.requireAuth, s.handlePageText)
	f.Post("/ingest", s.requireAuth, s.handleIngest)
	f.Post("/analyze", s.requireAuth, s.handleAnalyze)
	f.Post("/unknowns", s.requireAuth, s.handleAddUnknown)
	f.Get("/queue", s.requireAuth, s.handleQueue)
	f.Get("/export", s.requireAuth, s.handleExport)
	f.Post("/queue/clear", s.requireAuth, s.handleClearQueue)

	s.fiber = f
	return s, nil
}

// App exposes the underlying Fiber app (tests).
func (s *Server) App() *fiber.App { return s.fiber }

// Listen starts the HTTP server (blocking).
func (s *Server) Listen() error {
	return s.fiber.Listen(s.addr)
}

// Shutdown stops the server.
func (s *Server) Shutdown() error {
	return s.fiber.Shutdown()
}
