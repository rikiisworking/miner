package queuestore_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/ports"
)

func TestFile_CreateGetListUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	store := queuestore.NewFile(path)

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	e := ports.QueueEntry{
		ID:             "e1",
		Sentence:       "私は本を読む。",
		Unknowns:       []string{"本"},
		FirstUnknownAt: now,
	}
	if err := store.Create(e); err != nil {
		t.Fatal(err)
	}

	got, ok, err := store.Get("e1")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Sentence != e.Sentence || len(got.Unknowns) != 1 || got.Unknowns[0] != "本" {
		t.Fatalf("Get mismatch: %+v", got)
	}
	if !got.FirstUnknownAt.Equal(now) {
		t.Fatalf("FirstUnknownAt=%v want %v", got.FirstUnknownAt, now)
	}

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("List len=%d want 1", len(list))
	}

	e.Unknowns = append(e.Unknowns, "読む")
	if err := store.Update(e); err != nil {
		t.Fatal(err)
	}
	got, ok, err = store.Get("e1")
	if err != nil || !ok {
		t.Fatal(err)
	}
	if len(got.Unknowns) != 2 || got.Unknowns[1] != "読む" {
		t.Fatalf("after update: %+v", got)
	}
}

func TestFile_SurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	s1 := queuestore.NewFile(path)
	if err := s1.Create(ports.QueueEntry{
		ID:             "persist",
		Sentence:       "今日は雨だ。",
		Unknowns:       []string{"雨"},
		FirstUnknownAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	s2 := queuestore.NewFile(path)
	list, err := s2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "persist" || list[0].Unknowns[0] != "雨" {
		t.Fatalf("reopen list=%+v", list)
	}
}

func TestFile_AppendUnknown_AtomicAndDedupe(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	store := queuestore.NewFile(path)
	if err := store.Create(ports.QueueEntry{
		ID:             "e1",
		Sentence:       "s",
		Unknowns:       []string{"A"},
		FirstUnknownAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	e, added, found, err := store.AppendUnknown("e1", "B")
	if err != nil || !found || !added {
		t.Fatalf("append B: added=%v found=%v err=%v", added, found, err)
	}
	if len(e.Unknowns) != 2 {
		t.Fatalf("unknowns=%v", e.Unknowns)
	}

	e, added, found, err = store.AppendUnknown("e1", "B")
	if err != nil || !found || added {
		t.Fatalf("dup B: added=%v found=%v err=%v", added, found, err)
	}
	if len(e.Unknowns) != 2 {
		t.Fatalf("dup changed unknowns: %v", e.Unknowns)
	}

	_, _, found, err = store.AppendUnknown("missing", "X")
	if err != nil || found {
		t.Fatalf("missing: found=%v err=%v", found, err)
	}
}

func TestFile_ClearAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	store := queuestore.NewFile(path)
	if err := store.Create(ports.QueueEntry{
		ID: "e1", Sentence: "s", Unknowns: []string{"A"}, FirstUnknownAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ClearAll(); err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("after clear len=%d", len(list))
	}
	// No-op when empty
	if err := store.ClearAll(); err != nil {
		t.Fatal(err)
	}
	// Survives reopen
	store2 := queuestore.NewFile(path)
	list, err = store2.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("reopen after clear len=%d", len(list))
	}
}
