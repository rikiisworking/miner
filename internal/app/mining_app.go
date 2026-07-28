// Package app provides MiningApp, the application facade for product use-cases.
//
// Architectural rule: product rules live here (and behind ports this package owns).
// HTTP (Fiber) adapters must stay thin — map transport to MiningApp, do not re-implement
// analyze/queue/export/auth decisions in handlers. L1 tests exercise this package.
package app

import (
	"errors"
	"sync"
	"time"

	"github.com/rikiisworking/miner/internal/ports"
)

// Product sentinels (errors.Is friendly).
var (
	ErrInvalidPIN      = errors.New("invalid pin")
	ErrEmptySentence   = errors.New("empty sentence")
	ErrAnalyze         = errors.New("analysis failed")
	ErrEmptySurface    = errors.New("empty surface")
	ErrEntryNotFound   = errors.New("queue entry not found")
	ErrEmptyPage       = errors.New("empty page")
	ErrEmptyImage      = errors.New("empty image")
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrOcrFailed       = errors.New("ocr failed")
	ErrIngestBusy      = errors.New("ingest busy")
	ErrIngestCanceled  = errors.New("ingest canceled")
)

// MaxUploadBytes is the product cap for photo ingest (ticket 06 / spec): 10 MiB.
// HTTP BodyLimit and L1 IngestPage share this number; ocrtest asserts fixtures against it.
const MaxUploadBytes = 10 * 1024 * 1024

// MaxIngestDuration bounds one IngestPage OCR wait (product single-flight ceiling).
// Nested with adapter timeouts; the earlier deadline wins.
const MaxIngestDuration = 60 * time.Second

// MiningApp is the application facade for product use-cases.
// HTTP adapters stay thin over this type.
//
// Required ports are non-nil; NewMiningApp panics on nil (programmer error).
type MiningApp struct {
	pinAuth  ports.PinAuth
	analyzer ports.JapaneseAnalyzer
	queue    ports.QueueStore
	ocr      ports.OcrEngine

	// passes: ephemeral analyze-pass → entry binding (Pass protocol).
	passes passRegistry

	// ingestMu serializes IngestPage (single-flight). Separate from pass mu so
	// mark-unknown never waits on OCR I/O.
	ingestMu sync.Mutex
	// ingesting is true while IngestPage holds the single-flight slot.
	ingesting bool
}

// NewMiningApp constructs the facade with required ports.
// ocr must be non-nil (production: adapters/ocr.Tesseract).
// Nil ports panic — use errors only for recoverable config at HTTP layer.
func NewMiningApp(pinAuth ports.PinAuth, analyzer ports.JapaneseAnalyzer, queue ports.QueueStore, ocr ports.OcrEngine) *MiningApp {
	if pinAuth == nil {
		panic("pinAuth is required")
	}
	if analyzer == nil {
		panic("analyzer is required")
	}
	if queue == nil {
		panic("queue is required")
	}
	if ocr == nil {
		panic("ocr is required")
	}
	return &MiningApp{
		pinAuth:  pinAuth,
		analyzer: analyzer,
		queue:    queue,
		ocr:      ocr,
		passes:   newPassRegistry(),
	}
}

// Unlock verifies the shared PIN. On success the HTTP layer may establish a session.
// Domain layer does not own cookies or HTTP sessions.
func (m *MiningApp) Unlock(pin string) error {
	if !m.pinAuth.Verify(pin) {
		return ErrInvalidPIN
	}
	return nil
}

// IngestBusy reports whether IngestPage currently holds the single-flight slot.
// HTTP may use this to reject before buffering a large multipart body.
func (m *MiningApp) IngestBusy() bool {
	m.ingestMu.Lock()
	defer m.ingestMu.Unlock()
	return m.ingesting
}

func (m *MiningApp) clock() time.Time {
	return time.Now().UTC()
}
