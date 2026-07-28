package queuestore_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/ports"
)

func TestFile_CreateList(t *testing.T) {
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

	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("List len=%d want 1", len(list))
	}
	got := list[0]
	if got.Sentence != e.Sentence || len(got.Unknowns) != 1 || got.Unknowns[0] != "本" {
		t.Fatalf("List mismatch: %+v", got)
	}
	if !got.FirstUnknownAt.Equal(now) {
		t.Fatalf("FirstUnknownAt=%v want %v", got.FirstUnknownAt, now)
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

	res, err := store.AppendUnknown("e1", "B")
	if err != nil || !res.Found || !res.Added {
		t.Fatalf("append B: %+v err=%v", res, err)
	}
	if len(res.Entry.Unknowns) != 2 {
		t.Fatalf("unknowns=%v", res.Entry.Unknowns)
	}

	res, err = store.AppendUnknown("e1", "B")
	if err != nil || !res.Found || res.Added {
		t.Fatalf("dup B: %+v err=%v", res, err)
	}
	if len(res.Entry.Unknowns) != 2 {
		t.Fatalf("dup changed unknowns: %v", res.Entry.Unknowns)
	}

	res, err = store.AppendUnknown("missing", "X")
	if err != nil || res.Found {
		t.Fatalf("missing: %+v err=%v", res, err)
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

func TestFile_CreateRejectsEmptyAndDuplicateID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	store := queuestore.NewFile(path)
	if err := store.Create(ports.QueueEntry{ID: ""}); err == nil {
		t.Fatal("expected empty id error")
	}
	e := ports.QueueEntry{
		ID: "e1", Sentence: "s", Unknowns: []string{"A"}, FirstUnknownAt: time.Now().UTC(),
	}
	if err := store.Create(e); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(e); err == nil {
		t.Fatal("expected duplicate id error")
	}
}

func TestFile_AppendUnknown_EmptyID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	store := queuestore.NewFile(path)
	_, err := store.AppendUnknown("", "x")
	if err == nil {
		t.Fatal("expected empty id error")
	}
}

func TestFile_LoadEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	store := queuestore.NewFile(path)
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("empty file list=%+v", list)
	}
}

func TestFile_LoadCorruptJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := queuestore.NewFile(path)
	_, err := store.List()
	if err == nil {
		t.Fatal("expected decode error")
	}
}
