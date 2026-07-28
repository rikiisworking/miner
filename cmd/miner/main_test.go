package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWebFS_Embed(t *testing.T) {
	fsys, err := resolveWebFS("")
	if err != nil {
		t.Fatal(err)
	}
	// Embedded assets must expose templates used by httpapi.
	f, err := fsys.Open("templates/shell.html")
	if err != nil {
		t.Fatalf("open shell: %v", err)
	}
	_ = f.Close()
	f2, err := fsys.Open("static/camera.js")
	if err != nil {
		t.Fatalf("open camera.js: %v", err)
	}
	_ = f2.Close()
}

func TestResolveWebFS_DiskOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "templates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "templates", "x.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	fsys, err := resolveWebFS(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, err := fsys.Open("templates/x.html")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

func TestResolveWebFS_MissingPath(t *testing.T) {
	_, err := resolveWebFS(filepath.Join(t.TempDir(), "no-such-dir"))
	if err == nil {
		t.Fatal("want error for missing MINER_WEB_ROOT")
	}
}
