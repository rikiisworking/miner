package queuestore

import (
	"strings"

	"github.com/rikiisworking/miner/internal/ports"
)

// copyEntry deep-copies entry fields. Strings are cloned so callers that pass
// request-scoped buffers (e.g. Fiber FormValue) cannot corrupt stored data
// when the next request reuses the buffer.
func copyEntry(e ports.QueueEntry) ports.QueueEntry {
	out := ports.QueueEntry{
		ID:             strings.Clone(e.ID),
		Sentence:       strings.Clone(e.Sentence),
		FirstUnknownAt: e.FirstUnknownAt,
	}
	if len(e.Unknowns) == 0 {
		out.Unknowns = []string{}
		return out
	}
	out.Unknowns = make([]string, len(e.Unknowns))
	for i, u := range e.Unknowns {
		out.Unknowns[i] = strings.Clone(u)
	}
	return out
}

// appendSurfaceIfAbsent appends surface when not already present.
// Returns a deep-copied entry and whether the surface was newly added.
func appendSurfaceIfAbsent(e ports.QueueEntry, surface string) (ports.QueueEntry, bool) {
	for _, u := range e.Unknowns {
		if u == surface {
			return copyEntry(e), false
		}
	}
	cloned := copyEntry(e)
	// Clone surface: may be a view into a reused request body buffer.
	cloned.Unknowns = append(cloned.Unknowns, strings.Clone(surface))
	return cloned, true
}
