// Package queuestore provides durable QueueStore adapters.
package queuestore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rikiisworking/miner/internal/ports"
)

// File is a JSON-file QueueStore. Survives process restart; process-safe via mutex.
// Not multi-process safe beyond atomic rename of the data file.
type File struct {
	path string
	mu   sync.Mutex
}

// NewFile creates a QueueStore backed by path (created on first write).
func NewFile(path string) *File {
	return &File{path: path}
}

type fileDoc struct {
	Entries []ports.QueueEntry `json:"entries"`
}

// Create implements ports.QueueStore.
func (f *File) Create(entry ports.QueueEntry) error {
	if entry.ID == "" {
		return ErrEmptyID
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, err := f.load()
	if err != nil {
		return err
	}
	for _, e := range doc.Entries {
		if e.ID == entry.ID {
			return fmt.Errorf("%w: %q", ErrDuplicateID, entry.ID)
		}
	}
	doc.Entries = append(doc.Entries, copyEntry(entry))
	return f.save(doc)
}

// List implements ports.QueueStore.
func (f *File) List() ([]ports.QueueEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, err := f.load()
	if err != nil {
		return nil, err
	}
	out := make([]ports.QueueEntry, len(doc.Entries))
	for i, e := range doc.Entries {
		out[i] = copyEntry(e)
	}
	return out, nil
}

// ClearAll implements ports.QueueStore.
func (f *File) ClearAll() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.save(fileDoc{Entries: []ports.QueueEntry{}})
}

// AppendUnknown implements ports.QueueStore — atomic read-modify-write under one lock.
func (f *File) AppendUnknown(id, surface string) (ports.AppendResult, error) {
	if id == "" {
		return ports.AppendResult{}, ErrEmptyID
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, err := f.load()
	if err != nil {
		return ports.AppendResult{}, err
	}
	for i, e := range doc.Entries {
		if e.ID != id {
			continue
		}
		next, added := appendSurfaceIfAbsent(e, surface)
		if !added {
			return ports.AppendResult{Entry: next, Added: false, Found: true}, nil
		}
		doc.Entries[i] = next
		if err := f.save(doc); err != nil {
			return ports.AppendResult{Found: true}, err
		}
		return ports.AppendResult{Entry: next, Added: true, Found: true}, nil
	}
	return ports.AppendResult{Found: false}, nil
}

func (f *File) load() (fileDoc, error) {
	data, err := os.ReadFile(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return fileDoc{Entries: []ports.QueueEntry{}}, nil
		}
		return fileDoc{}, err
	}
	if len(data) == 0 {
		return fileDoc{Entries: []ports.QueueEntry{}}, nil
	}
	var doc fileDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return fileDoc{}, fmt.Errorf("queuestore: decode %s: %w", f.path, err)
	}
	if doc.Entries == nil {
		doc.Entries = []ports.QueueEntry{}
	}
	return doc, nil
}

func (f *File) save(doc fileDoc) error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}
