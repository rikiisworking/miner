package queuestore

import (
	"fmt"
	"sync"

	"github.com/rikiisworking/miner/internal/ports"
)

// Mem is an in-memory QueueStore for tests (and ephemeral use).
// Process-local only; not durable.
type Mem struct {
	mu    sync.Mutex
	byID  map[string]ports.QueueEntry
	order []string
}

// NewMem returns an empty in-memory QueueStore.
func NewMem() *Mem {
	return &Mem{byID: map[string]ports.QueueEntry{}}
}

// Create implements ports.QueueStore.
func (m *Mem) Create(entry ports.QueueEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry.ID == "" {
		return ErrEmptyID
	}
	if _, ok := m.byID[entry.ID]; ok {
		return fmt.Errorf("%w: %q", ErrDuplicateID, entry.ID)
	}
	m.byID[entry.ID] = copyEntry(entry)
	m.order = append(m.order, entry.ID)
	return nil
}

// List implements ports.QueueStore.
func (m *Mem) List() ([]ports.QueueEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ports.QueueEntry, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, copyEntry(m.byID[id]))
	}
	return out, nil
}

// AppendUnknown implements ports.QueueStore.
func (m *Mem) AppendUnknown(id, surface string) (ports.AppendResult, error) {
	if id == "" {
		return ports.AppendResult{}, ErrEmptyID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.byID[id]
	if !ok {
		return ports.AppendResult{Found: false}, nil
	}
	next, added := appendSurfaceIfAbsent(e, surface)
	m.byID[id] = next
	return ports.AppendResult{Entry: next, Added: added, Found: true}, nil
}

// ClearAll implements ports.QueueStore.
func (m *Mem) ClearAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID = map[string]ports.QueueEntry{}
	m.order = nil
	return nil
}
