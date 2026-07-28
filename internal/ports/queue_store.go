package ports

import "time"

// QueueEntry is a durable mining record: one mining pass with ordered unknowns.
// New mining pass always gets a new ID even if Sentence matches an existing entry.
type QueueEntry struct {
	// ID is a stable unique identifier for this entry.
	ID string
	// Sentence is the working sentence text at first unknown save.
	Sentence string
	// Unknowns are surface forms in first-tap order (unique within the entry).
	Unknowns []string
	// FirstUnknownAt is set when the entry first receives an unknown.
	FirstUnknownAt time.Time
}

// AppendResult is the outcome of QueueStore.AppendUnknown.
type AppendResult struct {
	Entry QueueEntry
	// Added is true when surface was newly appended (false if already present).
	Added bool
	// Found is false when id is missing from the store.
	Found bool
}

// QueueStore persists queue entries across process restart.
// Product surface is intentionally narrow: create, list, atomic append, clear.
// No Get/Update — MiningApp never needs generic CRUD; AppendUnknown owns mutation.
// Images and OCR text must not live here (ephemeral ingest only).
type QueueStore interface {
	// Create inserts a new entry. ID must be unique.
	Create(entry QueueEntry) error
	// List returns all entries (order not required). Export sorts by first-unknown-at.
	List() ([]QueueEntry, error)
	// AppendUnknown atomically appends surface if absent (single locked RMW).
	AppendUnknown(id, surface string) (AppendResult, error)
	// ClearAll deletes every entry. No-op when already empty.
	ClearAll() error
}
