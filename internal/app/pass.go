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

func (p *passRegistry) clear() {
	p.mu.Lock()
	p.byID = map[string]string{}
	p.mu.Unlock()
}
