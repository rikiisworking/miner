package app

import (
	"sort"
	"strings"

	"github.com/rikiisworking/miner/internal/ports"
)

// ListQueue returns all durable queue entries in product display order:
// first-unknown-at ascending, tie-break entry id ascending (same as export).
func (m *MiningApp) ListQueue() ([]ports.QueueEntry, error) {
	list, err := m.queue.List()
	if err != nil {
		return nil, err
	}
	if list == nil {
		return []ports.QueueEntry{}, nil
	}
	return sortQueueEntries(list), nil
}

// ExportMarkdown builds a UTF-8 Markdown nested list of exportable queue entries.
// Only entries with ≥1 unknown are included. Order: first-unknown-at ascending,
// tie-break entry id ascending. Unknowns keep first-tap order.
// Export never mutates the queue. Empty queue yields an empty document (no error).
func (m *MiningApp) ExportMarkdown() (string, error) {
	list, err := m.queue.List()
	if err != nil {
		return "", err
	}
	exportable := make([]ports.QueueEntry, 0, len(list))
	for _, e := range list {
		if len(e.Unknowns) >= 1 {
			exportable = append(exportable, e)
		}
	}
	exportable = sortQueueEntries(exportable)

	if len(exportable) == 0 {
		return "", nil
	}

	var b strings.Builder
	for _, e := range exportable {
		b.WriteString("- ")
		b.WriteString(flattenListText(e.Sentence))
		b.WriteByte('\n')
		for _, u := range e.Unknowns {
			b.WriteString("  - ")
			b.WriteString(flattenListText(u))
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

// flattenListText keeps Markdown list structure intact: newlines become spaces.
func flattenListText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// ClearAll wipes every durable queue entry. No-op when already empty.
// Also drops ephemeral pass→entry bindings under the same lock as first-tap
// create/bind so concurrent mark-unknown cannot rebind a deleted entry.
func (m *MiningApp) ClearAll() error {
	return m.passes.clearWith(func() error {
		return m.queue.ClearAll()
	})
}

// sortQueueEntries orders by FirstUnknownAt ascending, then ID ascending.
func sortQueueEntries(list []ports.QueueEntry) []ports.QueueEntry {
	out := make([]ports.QueueEntry, len(list))
	copy(out, list)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.FirstUnknownAt.Equal(b.FirstUnknownAt) {
			return a.FirstUnknownAt.Before(b.FirstUnknownAt)
		}
		return a.ID < b.ID
	})
	return out
}
