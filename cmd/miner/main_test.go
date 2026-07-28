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
	for _, path := range []string{
		"templates/home.html",
		"templates/capture.html",
		"static/camera.js",
	} {
		f, err := fsys.Open(path)
		if err != nil {
			t.Fatalf("open %s: %v", path, err)
		}
		_ = f.Close()
	}
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
