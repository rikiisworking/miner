package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotEnv_MissingFile(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Fatalf("missing file should be a no-op: %v", err)
	}
}

func TestLoadDotEnv_SetsAndDoesNotOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "" +
		"# comment\n" +
		"\n" +
		"MINER_PIN_TEST_A=from-file\n" +
		"export MINER_PIN_TEST_B='quoted'\n" +
		`MINER_PIN_TEST_C="double"` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MINER_PIN_TEST_A", "from-shell")
	// Ensure B/C are unset so file can set them.
	t.Setenv("MINER_PIN_TEST_B", "")
	_ = os.Unsetenv("MINER_PIN_TEST_B")
	_ = os.Unsetenv("MINER_PIN_TEST_C")

	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}

	if got := os.Getenv("MINER_PIN_TEST_A"); got != "from-shell" {
		t.Fatalf("existing env must win: got %q", got)
	}
	if got := os.Getenv("MINER_PIN_TEST_B"); got != "quoted" {
		t.Fatalf("B: want quoted, got %q", got)
	}
	if got := os.Getenv("MINER_PIN_TEST_C"); got != "double" {
		t.Fatalf("C: want double, got %q", got)
	}
}

func TestLoadDotEnv_EmptyValueAndEqualsInValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	keyEmpty := "MINER_ENV_EMPTY_" + t.Name()
	keyEq := "MINER_ENV_EQ_" + t.Name()
	keySpaces := "MINER_ENV_SP_" + t.Name()
	_ = os.Unsetenv(keyEmpty)
	_ = os.Unsetenv(keyEq)
	_ = os.Unsetenv(keySpaces)

	content := keyEmpty + "=\n" +
		keyEq + "=a=b=c\n" +
		"  " + keySpaces + "  =  spaced  \n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(keyEmpty); got != "" {
		t.Fatalf("empty value: got %q", got)
	}
	if got := os.Getenv(keyEq); got != "a=b=c" {
		t.Fatalf("equals in value: got %q", got)
	}
	if got := os.Getenv(keySpaces); got != "spaced" {
		t.Fatalf("trimmed spaces: got %q", got)
	}
}

func TestLoadDotEnv_InvalidLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("NOT_A_PAIR\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := loadDotEnv(path); err == nil {
		t.Fatal("want error for line without =")
	}
}

func TestLoadDotEnv_EmptyKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("=no-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := loadDotEnv(path)
	if err == nil {
		t.Fatal("want error for empty key")
	}
	if !strings.Contains(err.Error(), "empty key") {
		t.Fatalf("want empty key error, got %v", err)
	}
}

func TestLoadDotEnv_EmptyExistingEnvNotOverridden(t *testing.T) {
	// Empty-but-set env (LookupEnv true) must not be replaced by .env.
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	key := "MINER_ENV_KEEP_EMPTY_" + t.Name()
	if err := os.WriteFile(path, []byte(key+"=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(key, "")
	if err := loadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if got, ok := os.LookupEnv(key); !ok || got != "" {
		t.Fatalf("empty existing env must win: ok=%v got=%q", ok, got)
	}
}

func TestLoadDotEnv_ExampleFile(t *testing.T) {
	// .env.example is the supported template; must parse and supply MINER_PIN.
	root := repoRoot(t)
	example := filepath.Join(root, ".env.example")
	b, err := os.ReadFile(example)
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	if !strings.Contains(string(b), "MINER_PIN=") {
		t.Fatal(".env.example must document MINER_PIN=")
	}

	// Load a copy so we do not depend on process MINER_PIN state for the example value.
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	// Use a dedicated key from example content that is always present.
	// Clear MINER_PIN only if we will restore via t.Setenv pattern: save/restore.
	prev, had := os.LookupEnv("MINER_PIN")
	_ = os.Unsetenv("MINER_PIN")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("MINER_PIN", prev)
		} else {
			_ = os.Unsetenv("MINER_PIN")
		}
	})

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("load .env.example content: %v", err)
	}
	if got := os.Getenv("MINER_PIN"); got != "change-me" {
		t.Fatalf("MINER_PIN from example: got %q want change-me", got)
	}
}

func TestUnquoteEnvValue(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`plain`, `plain`},
		{`'x'`, `x`},
		{`"y"`, `y`},
		{`"`, `"`},
		{``, ``},
		{`''`, ``},
		{`""`, ``},
		{`'keep`, `'keep`},
	}
	for _, tc := range cases {
		if got := unquoteEnvValue(tc.in); got != tc.want {
			t.Fatalf("unquote(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
