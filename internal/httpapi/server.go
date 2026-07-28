package httpapi

import (
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/session"
	"github.com/gofiber/storage/memory/v2"

	"github.com/rikiisworking/miner/internal/app"
)

const (
	sessionKeyAuth = "authenticated"
	cookieName     = "miner_session"
	// multipartOverhead is BodyLimit headroom above MaxUploadBytes for framing.
	multipartOverhead = 512 * 1024

	// unlockWindow / unlockMaxFails throttle PIN guessing on the LAN.
	unlockWindow   = time.Minute
	unlockMaxFails = 10

	// Session lives in memory until process restart; cookie max-age is long so a
	// long-running home server does not force re-PIN mid-session (CONTEXT).
	sessionExpiration = 365 * 24 * time.Hour
)

// Server is the Fiber HTTP adapter over MiningApp.
type Server struct {
	mining    *app.MiningApp
	fiber     *fiber.App
	store     *session.Store
	templates *template.Template
	addr      string

	unlockMu    sync.Mutex
	unlockFails map[string][]time.Time
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
		// Cookie is session-scoped to process lifetime for practical home use;
		// in-memory store still drops all sessions on process restart.
		KeyLookup:      "cookie:" + cookieName,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   false,
		CookiePath:     "/",
		Expiration:     sessionExpiration,
	})

	s := &Server{
		mining:      cfg.MiningApp,
		store:       sess,
		templates:   tmpl,
		addr:        addr,
		unlockFails: map[string][]time.Time{},
	}

	// BodyLimit must allow product MaxUploadBytes images plus multipart framing.
	// Semantic oversize still rejected in MiningApp.IngestPage (L1).
	f := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             app.MaxUploadBytes + multipartOverhead,
		ErrorHandler:          s.errorHandler,
	})

	f.Use(recover.New(recover.Config{
		EnableStackTrace: false,
	}))

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

// errorHandler maps framework errors (e.g. BodyLimit 413) to HTMX-friendly HTML.
func (s *Server) errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var fe *fiber.Error
	if errors.As(err, &fe) {
		code = fe.Code
	}
	if code == fiber.StatusRequestEntityTooLarge {
		return s.renderCandidatesErr(c, code, msgImageTooLarge)
	}
	if c.Get("HX-Request") == "true" {
		return s.renderHTMXError(c, code, "Something went wrong. Try again.")
	}
	return c.Status(code).SendString(err.Error())
}

func (s *Server) unlockAllowed(ip string) bool {
	s.unlockMu.Lock()
	defer s.unlockMu.Unlock()
	now := time.Now()
	cutoff := now.Add(-unlockWindow)
	hits := s.unlockFails[ip]
	kept := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	s.unlockFails[ip] = kept
	return len(kept) < unlockMaxFails
}

func (s *Server) recordUnlockFail(ip string) {
	s.unlockMu.Lock()
	defer s.unlockMu.Unlock()
	s.unlockFails[ip] = append(s.unlockFails[ip], time.Now())
}

func (s *Server) clearUnlockFails(ip string) {
	s.unlockMu.Lock()
	defer s.unlockMu.Unlock()
	delete(s.unlockFails, ip)
}
