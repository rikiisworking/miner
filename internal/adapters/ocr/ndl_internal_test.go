package ocr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTruncate(t *testing.T) {
	if got := truncate("  hi  ", 10); got != "hi" {
		t.Fatalf("short: %q", got)
	}
	long := "abcdefghijKLMNOP"
	got := truncate(long, 5)
	if got != "abcde…" {
		t.Fatalf("long: %q", got)
	}
	if got := truncate("   ", 3); got != "" {
		t.Fatalf("blank: %q", got)
	}
}

func TestEnvTruthy(t *testing.T) {
	t.Setenv("MINER_TEST_ON", "1")
	t.Setenv("MINER_TEST_TRUE", "True")
	t.Setenv("MINER_TEST_YES", "yes")
	t.Setenv("MINER_TEST_ONW", "on")
	t.Setenv("MINER_TEST_OFF", "0")
	t.Setenv("MINER_TEST_EMPTY", "")
	for _, k := range []string{"MINER_TEST_ON", "MINER_TEST_TRUE", "MINER_TEST_YES", "MINER_TEST_ONW"} {
		if !envTruthy(k) {
			t.Fatalf("%s should be true", k)
		}
	}
	if envTruthy("MINER_TEST_OFF") || envTruthy("MINER_TEST_EMPTY") || envTruthy("MINER_TEST_MISSING") {
		t.Fatal("expected false")
	}
}

func TestFindDefaultWorker(t *testing.T) {
	dir := t.TempDir()
	scripts := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	worker := filepath.Join(scripts, defaultWorkerName)
	if err := os.WriteFile(worker, []byte("#!/usr/bin/env python3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	got := findDefaultWorker()
	if got == "" {
		t.Fatal("expected worker path")
	}
	abs, _ := filepath.Abs(worker)
	gotAbs, _ := filepath.Abs(got)
	if gotAbs != abs {
		t.Fatalf("got %q want %q", gotAbs, abs)
	}
}

func TestBytesBuffer(t *testing.T) {
	var b bytesBuffer
	n, err := b.Write([]byte("stderr-line"))
	if err != nil || n != 11 {
		t.Fatalf("write: n=%d err=%v", n, err)
	}
	if b.String() != "stderr-line" {
		t.Fatalf("string: %q", b.String())
	}
}
