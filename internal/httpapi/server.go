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
		app:       cfg.MiningApp,
		store:     sess,
		templates: tmpl,
		addr:      addr,
	}

	// BodyLimit must allow product MaxUploadBytes images plus multipart framing.
	// Semantic oversize still rejected in MiningApp.IngestPage (L1).
	const multipartOverhead = 512 * 1024
	f := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             app.MaxUploadBytes + multipartOverhead,
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

// handlePageText proposes sentence candidates from multi-sentence page paste (ticket 05).
// Ephemeral only — does not write the durable queue.
func (s *Server) handlePageText(c *fiber.Ctx) error {
	pageText := c.FormValue("page_text")
	cands, err := s.app.ProposeSentences(pageText)
	if err != nil {
		if errors.Is(err, app.ErrEmptyPage) {
			c.Status(fiber.StatusBadRequest)
			return s.render(c, "sentence_candidates", map[string]any{
				"Error":      "Enter page text to split into sentences.",
				"Candidates": nil,
			})
		}
		return err
	}
	return s.render(c, "sentence_candidates", map[string]any{
		"Error":      "",
		"Candidates": cands,
	})
}

func (s *Server) handleAnalyze(c *fiber.Ctx) error {
	sentence := c.FormValue("sentence")
	result, err := s.app.AnalyzeSentence(sentence)
	if err != nil {
		msg := "Analysis failed. Try again."
		status := fiber.StatusUnprocessableEntity
		if errors.Is(err, app.ErrEmptySentence) {
			msg = "Enter a sentence to analyze."
			status = fiber.StatusBadRequest
		} else if errors.Is(err, app.ErrAnalyze) {
			msg = "Analysis failed. The sentence could not be tokenized."
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

	res, err := s.app.AddUnknown(sentence, surface, entryID, passID)
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
		"Added":     res.Added,
		"Created":   res.Created,
	})
}

func (s *Server) handleQueue(c *fiber.Ctx) error {
	entries, err := s.app.ListQueue()
	if err != nil {
		return err
	}
	return s.render(c, "queue", map[string]any{
		"Entries": entries,
	})
}

func (s *Server) handleExport(c *fiber.Ctx) error {
	md, err := s.app.ExportMarkdown()
	if err != nil {
		return err
	}
	c.Set("Content-Type", "text/markdown; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="miner-export.md"`)
	return c.SendString(md)
}

func (s *Server) handleClearQueue(c *fiber.Ctx) error {
	if err := s.app.ClearAll(); err != nil {
		return err
	}
	// Prefer redirect so non-HTMX form POST lands on empty queue page.
	if c.Get("HX-Request") == "true" {
		entries, err := s.app.ListQueue()
		if err != nil {
			return err
		}
		return s.render(c, "queue", map[string]any{
			"Entries": entries,
		})
	}
	return c.Redirect("/queue", fiber.StatusSeeOther)
}

func (s *Server) requireAuth(c *fiber.Ctx) error {
	ok, err := s.isAuthenticated(c)
	if err != nil {
		return err
	}
	if !ok {
		// HTMX: generic auth fragment only — never a feature partial (analyze/queue/…).
		if c.Get("HX-Request") == "true" {
			c.Status(fiber.StatusUnauthorized)
			return s.render(c, "auth_error", map[string]any{
				"Error": "Session required. Enter PIN.",
			})
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
