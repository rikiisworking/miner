package queuestore_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/ports"
)

func TestMem_Contract(t *testing.T) {
	testQueueStoreContract(t, func(t *testing.T) ports.QueueStore {
		t.Helper()
		return queuestore.NewMem()
	})
}

func TestFile_Contract(t *testing.T) {
	testQueueStoreContract(t, func(t *testing.T) ports.QueueStore {
		t.Helper()
		return queuestore.NewFile(filepath.Join(t.TempDir(), "queue.json"))
	})
}

func testQueueStoreContract(t *testing.T, newStore func(t *testing.T) ports.QueueStore) {
	t.Helper()
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	t.Run("empty_list", func(t *testing.T) {
		s := newStore(t)
		list, err := s.List()
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 0 {
			t.Fatalf("list=%+v", list)
		}
	})

	t.Run("create_list_isolation", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ports.QueueEntry{
			ID: "e1", Sentence: "s", Unknowns: []string{"A"}, FirstUnknownAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		list, err := s.List()
		if err != nil || len(list) != 1 {
			t.Fatalf("list=%+v err=%v", list, err)
		}
		list[0].Unknowns[0] = "mutated"
		list2, err := s.List()
		if err != nil {
			t.Fatal(err)
		}
		if list2[0].Unknowns[0] != "A" {
			t.Fatalf("store leaked mutation: %v", list2[0].Unknowns)
		}
	})

	t.Run("append_add_dup_missing", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ports.QueueEntry{
			ID: "e1", Sentence: "s", Unknowns: []string{"A"}, FirstUnknownAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		res, err := s.AppendUnknown("e1", "B")
		if err != nil || !res.Found || !res.Added || len(res.Entry.Unknowns) != 2 {
			t.Fatalf("append B: %+v err=%v", res, err)
		}
		res, err = s.AppendUnknown("e1", "B")
		if err != nil || !res.Found || res.Added {
			t.Fatalf("dup B: %+v err=%v", res, err)
		}
		res, err = s.AppendUnknown("missing", "X")
		if err != nil || res.Found {
			t.Fatalf("missing: %+v err=%v", res, err)
		}
	})

	t.Run("clear_twice", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ports.QueueEntry{
			ID: "e1", Sentence: "s", Unknowns: []string{"A"}, FirstUnknownAt: now,
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.ClearAll(); err != nil {
			t.Fatal(err)
		}
		list, err := s.List()
		if err != nil || len(list) != 0 {
			t.Fatalf("after clear list=%+v err=%v", list, err)
		}
		if err := s.ClearAll(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty_id", func(t *testing.T) {
		s := newStore(t)
		if err := s.Create(ports.QueueEntry{ID: ""}); !errors.Is(err, queuestore.ErrEmptyID) {
			t.Fatalf("Create empty id: %v", err)
		}
		_, err := s.AppendUnknown("", "x")
		if !errors.Is(err, queuestore.ErrEmptyID) {
			t.Fatalf("Append empty id: %v", err)
		}
	})

	t.Run("duplicate_id", func(t *testing.T) {
		s := newStore(t)
		e := ports.QueueEntry{ID: "dup", Sentence: "s", Unknowns: []string{}, FirstUnknownAt: now}
		if err := s.Create(e); err != nil {
			t.Fatal(err)
		}
		if err := s.Create(e); !errors.Is(err, queuestore.ErrDuplicateID) {
			t.Fatalf("duplicate: %v", err)
		}
	})
}
