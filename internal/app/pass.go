package app

import (
	"strings"
	"sync"
)

// passRegistry maps ephemeral analyze pass IDs to durable queue entry IDs.
// Owns lock choreography for concurrent first-taps (one entry per pass).
type passRegistry struct {
	mu   sync.Mutex
	byID map[string]string // passID → entryID
}

func newPassRegistry() passRegistry {
	return passRegistry{byID: map[string]string{}}
}

// lookupOrCreate returns the bound entry for passID.
// If unbound, create runs under the lock; create must return a new entry id.
// passID is cloned when stored (Fiber FormValue lifetime).
func (p *passRegistry) lookupOrCreate(passID string, create func() (entryID string, err error)) (entryID string, created bool, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if eid, ok := p.byID[passID]; ok {
		return eid, false, nil
	}
	id, err := create()
	if err != nil {
		return "", false, err
	}
	p.byID[strings.Clone(passID)] = id
	return id, true, nil
}

// clearWith holds the registry lock for the whole fn, then empties the map.
// Used by ClearAll so queue wipe and pass unbind cannot race first-tap bind.
func (p *passRegistry) clearWith(fn func() error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := fn(); err != nil {
		return err
	}
	p.byID = map[string]string{}
	return nil
}

// drop removes one pass binding (stale entry after ClearAll races append).
func (p *passRegistry) drop(passID string) {
	p.mu.Lock()
	delete(p.byID, passID)
	p.mu.Unlock()
}
