package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the test binary's working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found from ", wd)
		}
		dir = parent
	}
}

func TestEnvExample_PresentAndGitignored(t *testing.T) {
	root := repoRoot(t)
	if _, err := os.Stat(filepath.Join(root, ".env.example")); err != nil {
		t.Fatalf(".env.example required: %v", err)
	}
	gi, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(gi)
	if !strings.Contains(text, ".env") {
		t.Fatal(".gitignore must ignore .env")
	}
	if !strings.Contains(text, "!.env.example") {
		t.Fatal(".gitignore must un-ignore .env.example")
	}
}

func TestMakefile_RunTunnelTarget(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, "run-tunnel:") {
		t.Fatal("Makefile must define run-tunnel target")
	}
	if !strings.Contains(text, "scripts/run_tunnel.sh") {
		t.Fatal("run-tunnel must invoke scripts/run_tunnel.sh")
	}
	// PIN may come from .env (not only export).
	if !strings.Contains(text, ".env") {
		t.Fatal("Makefile preflight should accept MINER_PIN from .env")
	}
}

func TestRunTunnelScript_BashSyntax(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "scripts", "run_tunnel.sh")
	cmd := exec.Command("bash", "-n", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash -n run_tunnel.sh: %v\n%s", err, out)
	}
}

func TestRunTunnelScript_CheckOnly_PortAndPIN(t *testing.T) {
	root := repoRoot(t)
	script := filepath.Join(root, "scripts", "run_tunnel.sh")

	// Missing PIN → fail before cloudflared (FORCE_NO_ENV ignores developer .env).
	t.Run("missing_pin", func(t *testing.T) {
		cmd := exec.Command("bash", script)
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"),
			"HOME=" + t.TempDir(),
			"MINER_TUNNEL_CHECK_ONLY=1",
			"MINER_TUNNEL_FORCE_NO_ENV=1",
			// Intentionally omit MINER_PIN so ambient export cannot satisfy the check.
		}
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("want failure without PIN, got success\n%s", out)
		}
		if !strings.Contains(string(out), "MINER_PIN") {
			t.Fatalf("want MINER_PIN hint in output:\n%s", out)
		}
	})

	t.Run("parses_addr_and_ok", func(t *testing.T) {
		// Need a fake cloudflared + fake bin/miner OR check-only skips those after pin.
		cmd := exec.Command("bash", script)
		cmd.Env = append(os.Environ(),
			"MINER_PIN=test-pin-for-check",
			"MINER_ADDR=:9876",
			"MINER_TUNNEL_CHECK_ONLY=1",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("check-only with PIN should succeed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "http://127.0.0.1:9876") {
			t.Fatalf("want local URL with parsed port in output:\n%s", out)
		}
	})

	t.Run("bad_addr", func(t *testing.T) {
		cmd := exec.Command("bash", script)
		cmd.Env = append(os.Environ(),
			"MINER_PIN=test-pin-for-check",
			"MINER_ADDR=not-a-port",
			"MINER_TUNNEL_CHECK_ONLY=1",
		)
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("want failure for bad MINER_ADDR\n%s", out)
		}
	})
}

func TestInstallNDLOCRScript_BashSyntax(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "scripts", "install_ndlocr.sh")
	cmd := exec.Command("bash", "-n", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash -n install_ndlocr.sh: %v\n%s", err, out)
	}
}

func TestInstallNDLOCR_PythonSupportedHelper(t *testing.T) {
	// Source script without running main (see BASH_SOURCE guard) and exercise helpers.
	root := repoRoot(t)
	script := filepath.Join(root, "scripts", "install_ndlocr.sh")
	body := `
set -euo pipefail
# shellcheck disable=SC1090
source "$1"
python_mm() { printf '%s\n' "$1"; }
python_supported 3.10
python_supported 3.11
python_supported 3.12
if python_supported 3.14; then echo "3.14 should fail"; exit 1; fi
if python_supported 3.9; then echo "3.9 should fail"; exit 1; fi
if python_supported ""; then echo "empty should fail"; exit 1; fi
echo ok
`
	cmd := exec.Command("bash", "-c", body, "bash", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python_supported checks failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("unexpected output: %s", out)
	}
}
