// Package ocrtest loads synthetic OCR page fixtures from testdata/ocr.
//
// L1 product rules should still use a fake OcrEngine. These fixtures support
// L2 multipart smoke, L3 upload journeys, and optional real-engine contract tests.
package ocrtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Case is one row from testdata/ocr/cases.json.
// Unknown JSON fields (e.g. notes) are ignored.
type Case struct {
	ID           string   `json:"id"`
	File         string   `json:"file"` // relative to testdata/ocr
	ExpectedText string   `json:"expected_text"`
	WantSuccess  bool     `json:"want_success"`
	MinOverlap   *float64 `json:"min_overlap"`
	Tags         []string `json:"tags"`

	root string // absolute testdata/ocr dir
}

// Path returns the absolute path to the image (or .bin) payload.
func (c Case) Path() string {
	return filepath.Join(c.root, filepath.FromSlash(c.File))
}

// Bytes reads the fixture file.
func (c Case) Bytes() ([]byte, error) {
	return os.ReadFile(c.Path())
}

// HasTag reports whether tag is listed on the case.
func (c Case) HasTag(tag string) bool {
	for _, t := range c.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// Manifest is the full cases.json document.
// Version/description may appear in JSON but are not required by Go readers.
type Manifest struct {
	MaxUploadBytes int    `json:"max_upload_bytes"`
	Cases          []Case `json:"cases"`
}

// byID returns the case or false.
func (m *Manifest) byID(id string) (Case, bool) {
	for _, c := range m.Cases {
		if c.ID == id {
			return c, true
		}
	}
	return Case{}, false
}

// Must returns the case or panics (for table tests with fixed ids).
func (m *Manifest) Must(id string) Case {
	c, ok := m.byID(id)
	if !ok {
		panic(fmt.Sprintf("ocrtest: unknown case id %q", id))
	}
	return c
}

// WithTag returns cases that include tag.
func (m *Manifest) WithTag(tag string) []Case {
	var out []Case
	for _, c := range m.Cases {
		if c.HasTag(tag) {
			out = append(out, c)
		}
	}
	return out
}

// HappyPath returns cases tagged "happy" (strong expected_text contracts).
func (m *Manifest) HappyPath() []Case {
	return m.WithTag("happy")
}

var (
	loadOnce sync.Once
	loaded   *Manifest
	loadErr  error
)

func fixturesDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("ocrtest: runtime.Caller failed")
	}
	// internal/ocrtest -> repo root -> testdata/ocr
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "testdata", "ocr"))
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return "", fmt.Errorf("ocrtest: fixtures dir missing: %s", root)
	}
	return root, nil
}

// Load reads cases.json once and attaches absolute paths.
func Load() (*Manifest, error) {
	loadOnce.Do(func() {
		root, err := fixturesDir()
		if err != nil {
			loadErr = err
			return
		}
		raw, err := os.ReadFile(filepath.Join(root, "cases.json"))
		if err != nil {
			loadErr = err
			return
		}
		var m Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			loadErr = err
			return
		}
		for i := range m.Cases {
			m.Cases[i].root = root
		}
		loaded = &m
	})
	return loaded, loadErr
}
