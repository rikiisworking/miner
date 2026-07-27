// Package app provides MiningApp, the application facade for product use-cases.
//
// Architectural rule: product rules live here (and behind ports this package owns).
// HTTP (Fiber) adapters must stay thin — map transport to MiningApp, do not re-implement
// analyze/queue/export/auth decisions in handlers. L1 tests exercise this package.
package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rikiisworking/miner/internal/ports"
)

// ErrInvalidPIN is returned when Unlock is called with a wrong PIN.
var ErrInvalidPIN = errors.New("invalid pin")

// ErrEmptySentence is returned when AnalyzeSentence receives blank text.
var ErrEmptySentence = errors.New("empty sentence")

// ErrAnalyze is returned when the analyzer port fails (wrapped with cause).
var ErrAnalyze = errors.New("analysis failed")

// ErrEmptySurface is returned when AddUnknown receives a blank surface form.
var ErrEmptySurface = errors.New("empty surface")

// ErrEntryNotFound is returned when AddUnknown references an unknown entry id.
var ErrEntryNotFound = errors.New("queue entry not found")

// MiningApp is the application facade for product use-cases.
// HTTP adapters stay thin over this type.
type MiningApp struct {
	pinAuth  ports.PinAuth
	analyzer ports.JapaneseAnalyzer
	queue    ports.QueueStore

	// mu guards openPasses (ephemeral analyze-pass → entry binding).
	mu sync.Mutex
	// openPasses maps pass id (from AnalyzeSentence) to queue entry id after first unknown.
	// Prevents concurrent empty-entry_id taps from creating multiple entries for one pass.
	openPasses map[string]string

	// now is injectable for tests; nil means time.Now.
	now func() time.Time
	// newID is injectable for tests; nil means random hex id.
	newID func() (string, error)
}

// NewMiningApp constructs the facade with required ports.
func NewMiningApp(pinAuth ports.PinAuth, analyzer ports.JapaneseAnalyzer, queue ports.QueueStore) *MiningApp {
	if pinAuth == nil {
		panic("pinAuth is required")
	}
	if analyzer == nil {
		panic("analyzer is required")
	}
	if queue == nil {
		panic("queue is required")
	}
	return &MiningApp{
		pinAuth:    pinAuth,
		analyzer:   analyzer,
		queue:      queue,
		openPasses: map[string]string{},
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

// SentenceAnalysis is the result of AnalyzeSentence: full token stream for furigana
// plus the filtered content-word list for the vocab rows.
type SentenceAnalysis struct {
	Sentence     string
	Tokens       []ports.Token
	ContentWords []ports.Token
	// PassID is an ephemeral id for this analyze result. First AddUnknown with this
	// pass creates the queue entry; later adds with the same pass append. Not durable.
	PassID string
}

// AnalyzeSentence runs the JapaneseAnalyzer and applies the content-word filter.
// Furigana uses Tokens; the list under the sentence uses ContentWords only.
// Analyze alone never writes the queue.
func (m *MiningApp) AnalyzeSentence(text string) (SentenceAnalysis, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return SentenceAnalysis{}, ErrEmptySentence
	}

	tokens, err := m.analyzer.Analyze(text)
	if err != nil {
		return SentenceAnalysis{}, fmt.Errorf("%w: %v", ErrAnalyze, err)
	}
	if tokens == nil {
		tokens = []ports.Token{}
	}

	passID, err := m.generateID()
	if err != nil {
		return SentenceAnalysis{}, err
	}

	return SentenceAnalysis{
		Sentence:     text,
		Tokens:       tokens,
		ContentWords: filterContentWords(tokens),
		PassID:       passID,
	}, nil
}

// filterContentWords keeps content tokens with a non-empty surface.
// Baseline product rule is encoded on Token.Content by the analyzer (or test fake).
func filterContentWords(tokens []ports.Token) []ports.Token {
	out := make([]ports.Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Content && t.Surface != "" {
			out = append(out, t)
		}
	}
	return out
}

// AddUnknownResult describes the outcome of AddUnknown.
type AddUnknownResult struct {
	// EntryID is the queue entry that owns this unknown (created or existing).
	EntryID string
	// Sentence is the entry sentence text.
	Sentence string
	// Surface is the unknown surface that was requested.
	Surface string
	// Created is true when this call created a new queue entry.
	Created bool
	// Added is true when the surface was newly appended (not a duplicate).
	Added bool
	// Duplicate is true when the surface was already on the entry (ignored).
	Duplicate bool
	// Unknowns is the entry's unknowns after this call (first-tap order).
	Unknowns []string
}

// AddUnknown saves a content-word surface as an unknown on a queue entry.
//
// Rules:
//   - empty surface rejected
//   - entryID set: atomic append (or duplicate) on that entry
//   - entryID empty + passID set: first call creates entry and binds pass→entry;
//     concurrent/later calls with same pass append (one entry per analyze pass)
//   - entryID empty + passID empty: always create new entry (explicit new mining pass)
//   - never merge by sentence text alone
func (m *MiningApp) AddUnknown(sentence, surface, entryID, passID string) (AddUnknownResult, error) {
	surface = strings.TrimSpace(surface)
	if surface == "" {
		return AddUnknownResult{}, ErrEmptySurface
	}
	sentence = strings.TrimSpace(sentence)
	entryID = strings.TrimSpace(entryID)
	passID = strings.TrimSpace(passID)

	if entryID != "" {
		return m.appendUnknown(entryID, surface)
	}
	if passID != "" {
		return m.addUnknownForPass(sentence, surface, passID)
	}
	return m.createEntryWithUnknown(sentence, surface)
}

func (m *MiningApp) addUnknownForPass(sentence, surface, passID string) (AddUnknownResult, error) {
	// Serialize create-vs-bind for this pass so concurrent first-taps share one entry.
	m.mu.Lock()
	if eid, ok := m.openPasses[passID]; ok {
		m.mu.Unlock()
		return m.appendUnknown(eid, surface)
	}
	// Create while holding mu so a second goroutine waits and then appends.
	res, err := m.createEntryWithUnknown(sentence, surface)
	if err != nil {
		m.mu.Unlock()
		return AddUnknownResult{}, err
	}
	m.openPasses[passID] = res.EntryID
	m.mu.Unlock()
	return res, nil
}

func (m *MiningApp) createEntryWithUnknown(sentence, surface string) (AddUnknownResult, error) {
	if sentence == "" {
		return AddUnknownResult{}, ErrEmptySentence
	}
	id, err := m.generateID()
	if err != nil {
		return AddUnknownResult{}, err
	}
	now := m.clock()
	entry := ports.QueueEntry{
		ID:             id,
		Sentence:       sentence,
		Unknowns:       []string{surface},
		FirstUnknownAt: now,
	}
	if err := m.queue.Create(entry); err != nil {
		return AddUnknownResult{}, err
	}
	return AddUnknownResult{
		EntryID:   id,
		Sentence:  sentence,
		Surface:   surface,
		Created:   true,
		Added:     true,
		Duplicate: false,
		Unknowns:  []string{surface},
	}, nil
}

func (m *MiningApp) appendUnknown(entryID, surface string) (AddUnknownResult, error) {
	entry, added, found, err := m.queue.AppendUnknown(entryID, surface)
	if err != nil {
		return AddUnknownResult{}, err
	}
	if !found {
		return AddUnknownResult{}, ErrEntryNotFound
	}
	unk := append([]string(nil), entry.Unknowns...)
	return AddUnknownResult{
		EntryID:   entry.ID,
		Sentence:  entry.Sentence,
		Surface:   surface,
		Created:   false,
		Added:     added,
		Duplicate: !added,
		Unknowns:  unk,
	}, nil
}

// ListQueue returns all durable queue entries.
func (m *MiningApp) ListQueue() ([]ports.QueueEntry, error) {
	list, err := m.queue.List()
	if err != nil {
		return nil, err
	}
	if list == nil {
		return []ports.QueueEntry{}, nil
	}
	return list, nil
}

func (m *MiningApp) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now().UTC()
}

func (m *MiningApp) generateID() (string, error) {
	if m.newID != nil {
		return m.newID()
	}
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate entry id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
