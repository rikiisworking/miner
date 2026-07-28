package app

import (
	"errors"
	"strings"

	"github.com/rikiisworking/miner/internal/ports"
)

// AddUnknownResult describes the outcome of AddUnknown.
type AddUnknownResult struct {
	// EntryID is the queue entry that owns this unknown (created or existing).
	EntryID string
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
		res, err := m.appendUnknown(entryID, surface)
		// After Clear all, Mine UI may still hold a stale entry_id via OOB swap.
		// Prefer pass_id heal so multi-tab clear+mark does not 404 until re-analyze.
		if err != nil && errors.Is(err, ErrEntryNotFound) && passID != "" {
			return m.addUnknownForPass(sentence, surface, passID)
		}
		return res, err
	}
	if passID != "" {
		return m.addUnknownForPass(sentence, surface, passID)
	}
	return m.createEntryWithUnknown(sentence, surface)
}

func (m *MiningApp) addUnknownForPass(sentence, surface, passID string) (AddUnknownResult, error) {
	// Retry: ClearAll can delete the entry after lookupOrCreate returns and
	// before appendUnknown (append runs outside the registry lock).
	for attempt := 0; attempt < 3; attempt++ {
		var createdRes AddUnknownResult
		entryID, created, err := m.passes.lookupOrCreate(passID, func() (string, error) {
			res, err := m.createEntryWithUnknown(sentence, surface)
			if err != nil {
				return "", err
			}
			createdRes = res
			return res.EntryID, nil
		})
		if err != nil {
			return AddUnknownResult{}, err
		}
		if created {
			return createdRes, nil
		}
		res, err := m.appendUnknown(entryID, surface)
		if err != nil && errors.Is(err, ErrEntryNotFound) {
			m.passes.drop(passID)
			continue
		}
		return res, err
	}
	return AddUnknownResult{}, ErrEntryNotFound
}

func (m *MiningApp) createEntryWithUnknown(sentence, surface string) (AddUnknownResult, error) {
	if sentence == "" {
		return AddUnknownResult{}, ErrEmptySentence
	}
	id, err := m.newID()
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
		Surface:   surface,
		Created:   true,
		Added:     true,
		Duplicate: false,
		Unknowns:  []string{surface},
	}, nil
}

func (m *MiningApp) appendUnknown(entryID, surface string) (AddUnknownResult, error) {
	res, err := m.queue.AppendUnknown(entryID, surface)
	if err != nil {
		return AddUnknownResult{}, err
	}
	if !res.Found {
		return AddUnknownResult{}, ErrEntryNotFound
	}
	unk := append([]string(nil), res.Entry.Unknowns...)
	return AddUnknownResult{
		EntryID:   res.Entry.ID,
		Surface:   surface,
		Created:   false,
		Added:     res.Added,
		Duplicate: !res.Added,
		Unknowns:  unk,
	}, nil
}
