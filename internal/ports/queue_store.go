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

// QueueStore persists queue entries across process restart.
type QueueStore interface {
	// Create inserts a new entry. ID must be unique.
	Create(entry QueueEntry) error
	// Update replaces an existing entry by ID.
	Update(entry QueueEntry) error
	// Get returns the entry for id, or false if missing.
	Get(id string) (QueueEntry, bool, error)
	// List returns all entries (order not required to be stable for product rules
	// beyond first-unknown-at for export, which is ticket 04).
	List() ([]QueueEntry, error)
	// AppendUnknown atomically appends surface if absent (single locked RMW).
	// found is false when id is missing. added is false when surface already present.
	AppendUnknown(id, surface string) (entry QueueEntry, added, found bool, err error)
}
