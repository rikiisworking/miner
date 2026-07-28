package queuestore_test

import (
	"testing"
	"time"

	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/ports"
)

func TestMem_CreateListAppendClear(t *testing.T) {
	m := queuestore.NewMem()
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if err := m.Create(ports.QueueEntry{
		ID: "e1", Sentence: "s", Unknowns: []string{"A"}, FirstUnknownAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	list, err := m.List()
	if err != nil || len(list) != 1 || list[0].Unknowns[0] != "A" {
		t.Fatalf("list=%+v err=%v", list, err)
	}

	res, err := m.AppendUnknown("e1", "B")
	if err != nil || !res.Found || !res.Added || len(res.Entry.Unknowns) != 2 {
		t.Fatalf("append B: %+v err=%v", res, err)
	}
	res, err = m.AppendUnknown("e1", "B")
	if err != nil || !res.Found || res.Added {
		t.Fatalf("dup B: %+v err=%v", res, err)
	}
	res, err = m.AppendUnknown("missing", "X")
	if err != nil || res.Found {
		t.Fatalf("missing: %+v err=%v", res, err)
	}

	if err := m.ClearAll(); err != nil {
		t.Fatal(err)
	}
	list, err = m.List()
	if err != nil || len(list) != 0 {
		t.Fatalf("after clear list=%+v err=%v", list, err)
	}
	if err := m.ClearAll(); err != nil {
		t.Fatal(err)
	}
}

func TestMem_CreateRejectsEmptyAndDuplicateID(t *testing.T) {
	m := queuestore.NewMem()
	if err := m.Create(ports.QueueEntry{ID: ""}); err == nil {
		t.Fatal("expected empty id error")
	}
	e := ports.QueueEntry{ID: "dup", Sentence: "s", Unknowns: []string{}, FirstUnknownAt: time.Now().UTC()}
	if err := m.Create(e); err != nil {
		t.Fatal(err)
	}
	if err := m.Create(e); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestMem_ListOrderAndIsolation(t *testing.T) {
	m := queuestore.NewMem()
	for _, id := range []string{"a", "b"} {
		if err := m.Create(ports.QueueEntry{
			ID: id, Sentence: id, Unknowns: []string{"u"}, FirstUnknownAt: time.Now().UTC(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "a" || list[1].ID != "b" {
		t.Fatalf("order=%+v", list)
	}
	// Mutating returned slice must not corrupt store.
	list[0].Unknowns[0] = "mutated"
	list2, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if list2[0].Unknowns[0] != "u" {
		t.Fatalf("store leaked mutation: %v", list2[0].Unknowns)
	}
}
