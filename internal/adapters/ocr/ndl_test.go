package ocr_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rikiisworking/miner/internal/adapters/ocr"
	"github.com/rikiisworking/miner/internal/ocrtest"
)

func TestNDL_EmptyImage(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerOK)
	_, err := eng.Recognize(context.Background(), nil)
	if !errors.Is(err, ocr.ErrEmptyImage) {
		t.Fatalf("got %v want ErrEmptyImage", err)
	}
}

func TestNDL_NotFound_MissingRoot(t *testing.T) {
	_, err := ocr.NewNDL(ocr.NDLConfig{
		Python: "python3",
		Worker: fakeWorkerPath(t, fakeWorkerOK),
		Root:   filepath.Join(t.TempDir(), "no-such-root"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ocr.ErrNDLNotFound) {
		t.Fatalf("got %v want ErrNDLNotFound", err)
	}
}

func TestNDL_FakeWorker_OK(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerOK)
	text, err := eng.Recognize(context.Background(), png1x1())
	if err != nil {
		t.Fatal(err)
	}
	if text != "病院に行った。" {
		t.Fatalf("got %q", text)
	}
}

func TestNDL_FakeWorker_Error(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerFail)
	_, err := eng.Recognize(context.Background(), png1x1())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ocr.ErrRecognizeFailed) {
		t.Fatalf("got %v want ErrRecognizeFailed", err)
	}
}

func TestNDL_Cancel(t *testing.T) {
	// Slow fake: sleep before responding so cancel can win.
	eng := newFakeNDL(t, fakeWorkerSlow)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := eng.Recognize(ctx, png1x1())
	if err == nil {
		t.Fatal("expected cancel/deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		// May also surface as recognize failed if worker races; accept cancel family.
		if !errors.Is(err, ocr.ErrRecognizeFailed) {
			t.Fatalf("got %v", err)
		}
	}
}

func TestNDL_NilAndCanceledContext(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerOK)
	// nil ctx treated as Background
	text, err := eng.Recognize(nil, png1x1()) //nolint:staticcheck // intentional nil ctx
	if err != nil {
		t.Fatal(err)
	}
	if text != "病院に行った。" {
		t.Fatalf("got %q", text)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = eng.Recognize(ctx, png1x1())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v want Canceled", err)
	}
}

func TestNDL_ReuseWorker_MultipleRecognizes(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerOK)
	for i := 0; i < 3; i++ {
		text, err := eng.Recognize(context.Background(), png1x1())
		if err != nil {
			t.Fatalf("i=%d: %v", i, err)
		}
		if text != "病院に行った。" {
			t.Fatalf("i=%d got %q", i, text)
		}
	}
}

func TestNDL_WorkerNoiseThenReady(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerNoise)
	text, err := eng.Recognize(context.Background(), png1x1())
	if err != nil {
		t.Fatal(err)
	}
	if text != "ok-after-noise" {
		t.Fatalf("got %q", text)
	}
}

func TestNDL_WorkerStaleIDThenMatch(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerStaleID)
	text, err := eng.Recognize(context.Background(), png1x1())
	if err != nil {
		t.Fatal(err)
	}
	if text != "matched" {
		t.Fatalf("got %q", text)
	}
}

func TestNDL_WorkerEmptyErrorMessage(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerFailEmpty)
	_, err := eng.Recognize(context.Background(), png1x1())
	if !errors.Is(err, ocr.ErrRecognizeFailed) {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "unknown worker error") {
		t.Fatalf("want unknown worker error in %v", err)
	}
}

func TestNDL_WorkerDiesAfterReady(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerExitOnReq)
	_, err := eng.Recognize(context.Background(), png1x1())
	if err == nil {
		t.Fatal("expected error when worker exits mid-request")
	}
	if !errors.Is(err, ocr.ErrRecognizeFailed) {
		t.Fatalf("got %v want ErrRecognizeFailed", err)
	}
}

func TestNDL_WorkerRestartAfterExit(t *testing.T) {
	// First request answered then worker exits; second Recognize restarts worker.
	eng := newFakeNDL(t, fakeWorkerOnceThenExit)
	text, err := eng.Recognize(context.Background(), png1x1())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if text != "once" {
		t.Fatalf("first text %q", text)
	}
	// Give process a moment to die so next write fails and triggers restart.
	time.Sleep(50 * time.Millisecond)
	text, err = eng.Recognize(context.Background(), png1x1())
	if err != nil {
		t.Fatalf("second after restart: %v", err)
	}
	if text != "once" {
		t.Fatalf("second text %q", text)
	}
}

func TestNDL_ReadyTimeout(t *testing.T) {
	root := fakeRoot(t)
	worker := fakeWorkerPath(t, fakeWorkerNeverReady)
	_, err := ocr.NewNDL(ocr.NDLConfig{
		Python:       pythonForFake(t),
		Worker:       worker,
		Root:         root,
		ReadyTimeout: 200 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected ready timeout")
	}
	if !errors.Is(err, ocr.ErrWorkerNotReady) {
		t.Fatalf("got %v want ErrWorkerNotReady", err)
	}
}

func TestNDL_WorkerExitsBeforeReady(t *testing.T) {
	root := fakeRoot(t)
	worker := fakeWorkerPath(t, fakeWorkerExitImmediate)
	_, err := ocr.NewNDL(ocr.NDLConfig{
		Python:       pythonForFake(t),
		Worker:       worker,
		Root:         root,
		ReadyTimeout: 2 * time.Second,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ocr.ErrWorkerNotReady) {
		t.Fatalf("got %v want ErrWorkerNotReady", err)
	}
}

func TestNDL_NotFound_MissingWorker(t *testing.T) {
	root := fakeRoot(t)
	_, err := ocr.NewNDL(ocr.NDLConfig{
		Python: pythonForFake(t),
		Worker: filepath.Join(t.TempDir(), "no-worker.py"),
		Root:   root,
	})
	if !errors.Is(err, ocr.ErrNDLNotFound) {
		t.Fatalf("got %v want ErrNDLNotFound", err)
	}
}

func TestNDL_NotFound_BadPython(t *testing.T) {
	root := fakeRoot(t)
	worker := fakeWorkerPath(t, fakeWorkerOK)
	_, err := ocr.NewNDL(ocr.NDLConfig{
		Python: filepath.Join(t.TempDir(), "no-such-python"),
		Worker: worker,
		Root:   root,
	})
	if !errors.Is(err, ocr.ErrNDLNotFound) {
		t.Fatalf("got %v want ErrNDLNotFound", err)
	}
}

func TestNDL_NotFound_PythonNameNotOnPATH(t *testing.T) {
	root := fakeRoot(t)
	worker := fakeWorkerPath(t, fakeWorkerOK)
	_, err := ocr.NewNDL(ocr.NDLConfig{
		Python: "python3-miner-definitely-not-installed-xyz",
		Worker: worker,
		Root:   root,
	})
	if !errors.Is(err, ocr.ErrNDLNotFound) {
		t.Fatalf("got %v want ErrNDLNotFound", err)
	}
}

func TestNDL_NotFound_EmptyRoot(t *testing.T) {
	_, err := ocr.NewNDL(ocr.NDLConfig{
		Python: pythonForFake(t),
		Worker: fakeWorkerPath(t, fakeWorkerOK),
		Root:   "",
	})
	if !errors.Is(err, ocr.ErrNDLNotFound) {
		t.Fatalf("got %v want ErrNDLNotFound", err)
	}
}

func TestNDL_NotFound_WorkerIsDir(t *testing.T) {
	root := fakeRoot(t)
	_, err := ocr.NewNDL(ocr.NDLConfig{
		Python: pythonForFake(t),
		Worker: t.TempDir(),
		Root:   root,
	})
	if !errors.Is(err, ocr.ErrNDLNotFound) {
		t.Fatalf("got %v want ErrNDLNotFound", err)
	}
}

func TestNDL_DiscoverDefaultWorker(t *testing.T) {
	// Place scripts/ndl_ocr_worker.py under a temp cwd; leave Worker empty.
	dir := t.TempDir()
	scripts := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	// Real enough fake worker for protocol.
	body := `#!/usr/bin/env python3
import sys, json
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    req=json.loads(line)
    print(json.dumps({"id": req.get("id",""), "ok": True, "text": "discovered"}), flush=True)
`
	if err := os.WriteFile(filepath.Join(scripts, "ndl_ocr_worker.py"), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	root := fakeRoot(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	eng, err := ocr.NewNDL(ocr.NDLConfig{
		Python:       pythonForFake(t),
		Worker:       "", // discover
		Root:         root,
		ReadyTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	text, err := eng.Recognize(context.Background(), png1x1())
	if err != nil {
		t.Fatal(err)
	}
	if text != "discovered" {
		t.Fatalf("got %q", text)
	}
}

func TestNDL_EnableTCY_Constructs(t *testing.T) {
	root := fakeRoot(t)
	worker := fakeWorkerPath(t, fakeWorkerOK)
	eng, err := ocr.NewNDL(ocr.NDLConfig{
		Python:       pythonForFake(t),
		Worker:       worker,
		Root:         root,
		EnableTCY:    true,
		Device:       "cpu",
		ReadyTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	if _, err := eng.Recognize(context.Background(), png1x1()); err != nil {
		t.Fatal(err)
	}
}

func TestNewNDLFromEnv_Missing(t *testing.T) {
	t.Setenv("MINER_NDL_ROOT", "")
	t.Setenv("MINER_NDL_WORKER", "")
	t.Setenv("MINER_NDL_PYTHON", "")
	_, err := ocr.NewNDLFromEnv()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ocr.ErrNDLNotFound) {
		t.Fatalf("got %v want ErrNDLNotFound", err)
	}
}

func TestNewNDLFromEnv_WithFake(t *testing.T) {
	root := fakeRoot(t)
	worker := fakeWorkerPath(t, fakeWorkerOK)
	t.Setenv("MINER_NDL_ROOT", root)
	t.Setenv("MINER_NDL_WORKER", worker)
	t.Setenv("MINER_NDL_PYTHON", pythonForFake(t))
	t.Setenv("MINER_NDL_DEVICE", "cpu")
	t.Setenv("MINER_NDL_ENABLE_TCY", "1")
	eng, err := ocr.NewNDLFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	text, err := eng.Recognize(context.Background(), png1x1())
	if err != nil {
		t.Fatal(err)
	}
	if text != "病院に行った。" {
		t.Fatalf("got %q", text)
	}
}

func TestMustEngine_WithFakeEnv(t *testing.T) {
	root := fakeRoot(t)
	worker := fakeWorkerPath(t, fakeWorkerOK)
	t.Setenv("MINER_NDL_ROOT", root)
	t.Setenv("MINER_NDL_WORKER", worker)
	t.Setenv("MINER_NDL_PYTHON", pythonForFake(t))
	eng := ocr.MustEngine(t)
	text, err := eng.Recognize(context.Background(), png1x1())
	if err != nil {
		t.Fatal(err)
	}
	if text != "病院に行った。" {
		t.Fatalf("got %q", text)
	}
}

func TestNDL_CloseIdempotent(t *testing.T) {
	eng := newFakeNDL(t, fakeWorkerOK)
	if err := eng.Close(); err != nil {
		t.Fatal(err)
	}
	if err := eng.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRuneOverlap_Helpers(t *testing.T) {
	if runeOverlap("", "") != 1 {
		t.Fatal("empty")
	}
	if runeOverlap("あ", "") != 0 {
		t.Fatal("want empty")
	}
	if runeOverlap("私は本を読む", "私は本を読む") != 1 {
		t.Fatal("exact")
	}
	if runeOverlap("私は本", "私は本を読む") < 0.4 {
		t.Fatal("partial")
	}
}

func TestNDL_SmokeSingleSentence(t *testing.T) {
	eng := newRealNDL(t)
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	img, err := m.Must("01_single_sentence").Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text, err := eng.Recognize(context.Background(), img)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	got := strings.ReplaceAll(text, " ", "")
	if !strings.Contains(got, "私は本を読む") {
		t.Fatalf("got %q want contain 私は本を読む", text)
	}
}

func TestNDL_SmokeMultiSentence(t *testing.T) {
	eng := newRealNDL(t)
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	img, err := m.Must("02_multi_sentence").Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text, err := eng.Recognize(context.Background(), img)
	if err != nil {
		t.Fatalf("Recognize: %v", err)
	}
	got := strings.ReplaceAll(text, " ", "")
	if !strings.Contains(got, "病院に行った") || !strings.Contains(got, "私は本を読む") {
		t.Fatalf("multi got %q", text)
	}
}

func TestNDL_NotAnImageFailsOrEmpty(t *testing.T) {
	eng := newRealNDL(t)
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	img, err := m.Must("19_not_an_image").Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text, err := eng.Recognize(context.Background(), img)
	if err != nil {
		if !errors.Is(err, ocr.ErrRecognizeFailed) {
			t.Fatalf("unexpected err type: %v", err)
		}
		return
	}
	if strings.TrimSpace(text) != "" {
		t.Logf("not_an_image returned text=%q (tolerated)", text)
	}
}

// contractSoftIDs: known-weak under NDLOCR-Lite on synthetic phone-stress fixtures.
// Still run for visibility; do not hard-fail on overlap (log only).
var contractSoftIDs = map[string]bool{
	"39_tilt_h_with_blur":             true,
	"44_brightness_mixed_lr":          true,
	"55_mixed_brightness_colour_blur": true,
}

type contractSuite struct {
	name       string
	tag        string
	defaultMin float64
}

// TestNDLContract_StressSuites runs real OCR when MINER_OCR_CONTRACT=1.
func TestNDLContract_StressSuites(t *testing.T) {
	if os.Getenv("MINER_OCR_CONTRACT") != "1" {
		t.Skip("set MINER_OCR_CONTRACT=1 to run real-engine contract suites")
	}
	eng := newRealNDL(t)
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}

	suites := []contractSuite{
		{name: "happy", tag: "happy", defaultMin: 0.7},
		{name: "vertical", tag: "vertical", defaultMin: 0.25},
		{name: "blur", tag: "blur", defaultMin: 0.25},
		{name: "brightness", tag: "brightness", defaultMin: 0.25},
		{name: "font", tag: "font", defaultMin: 0.35},
		{name: "thickness", tag: "thickness", defaultMin: 0.35},
		{name: "colour", tag: "colour", defaultMin: 0.35},
	}

	for _, suite := range suites {
		suite := suite
		t.Run(suite.name, func(t *testing.T) {
			cases := m.WithTag(suite.tag)
			if len(cases) == 0 {
				t.Fatalf("no fixtures tagged %q", suite.tag)
			}
			ran := 0
			for _, c := range cases {
				if c.ExpectedText == "" {
					continue
				}
				c := c
				t.Run(c.ID, func(t *testing.T) {
					img, err := c.Bytes()
					if err != nil {
						t.Fatal(err)
					}
					got, err := eng.Recognize(context.Background(), img)
					if err != nil {
						if contractSoftIDs[c.ID] || !c.WantSuccess {
							t.Logf("soft: Recognize error (tolerated): %v", err)
							return
						}
						t.Fatalf("Recognize: %v", err)
					}
					min := suite.defaultMin
					if c.MinOverlap != nil {
						min = *c.MinOverlap
					}
					score := runeOverlap(got, c.ExpectedText)
					t.Logf("overlap=%.2f min=%.2f out_len=%d", score, min, len(strings.TrimSpace(got)))

					if contractSoftIDs[c.ID] {
						if score < min {
							t.Logf("soft below min: score=%.2f < %.2f (known weak fixture)", score, min)
						}
						return
					}
					if !c.WantSuccess {
						return
					}
					if score < min {
						t.Errorf("overlap=%.2f < %.2f\n got=%q\nwant=%q", score, min, got, c.ExpectedText)
					}
				})
				ran++
			}
			if ran == 0 {
				t.Fatalf("suite %s: no cases with expected_text", suite.name)
			}
		})
	}
}

func TestNDLContract_HappyPathOverlap(t *testing.T) {
	if os.Getenv("MINER_OCR_CONTRACT") != "1" {
		t.Skip("set MINER_OCR_CONTRACT=1 to run real-engine contract suite")
	}
	eng := newRealNDL(t)
	m, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.HappyPath() {
		if c.ExpectedText == "" || !c.WantSuccess {
			continue
		}
		c := c
		t.Run(c.ID, func(t *testing.T) {
			img, err := c.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			got, err := eng.Recognize(context.Background(), img)
			if err != nil {
				t.Fatalf("Recognize: %v", err)
			}
			min := 0.7
			if c.MinOverlap != nil {
				min = *c.MinOverlap
			}
			score := runeOverlap(got, c.ExpectedText)
			if score < min {
				t.Errorf("overlap=%.2f < %.2f\n got=%q\nwant=%q", score, min, got, c.ExpectedText)
			}
		})
	}
}

// --- helpers ---

const (
	fakeWorkerOK            = "ok"
	fakeWorkerFail          = "fail"
	fakeWorkerFailEmpty     = "fail-empty"
	fakeWorkerSlow          = "slow"
	fakeWorkerNoise         = "noise"
	fakeWorkerStaleID       = "stale-id"
	fakeWorkerExitOnReq     = "exit-on-req"
	fakeWorkerOnceThenExit  = "once-then-exit"
	fakeWorkerNeverReady    = "never-ready"
	fakeWorkerExitImmediate = "exit-immediate"
)

func fakeRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "ocr.py"), []byte("# fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func newFakeNDL(t *testing.T, mode string) *ocr.NDL {
	t.Helper()
	root := fakeRoot(t)
	worker := fakeWorkerPath(t, mode)
	eng, err := ocr.NewNDL(ocr.NDLConfig{
		Python:       pythonForFake(t),
		Worker:       worker,
		Root:         root,
		ReadyTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

func newRealNDL(t *testing.T) *ocr.NDL {
	t.Helper()
	eng, err := ocr.NewNDLFromEnv()
	if err != nil {
		t.Skipf("NDLOCR-Lite not configured: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng
}

func pythonForFake(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("MINER_NDL_PYTHON"); p != "" {
		return p
	}
	if p, err := exec.LookPath("python3"); err == nil {
		return p
	}
	t.Skip("python3 not found for fake worker")
	return ""
}

func fakeWorkerPath(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake_worker.py")
	var body string
	switch mode {
	case fakeWorkerFail:
		body = `#!/usr/bin/env python3
import sys, json
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    req=json.loads(line)
    print(json.dumps({"id": req.get("id",""), "ok": False, "error": "boom"}), flush=True)
`
	case fakeWorkerFailEmpty:
		body = `#!/usr/bin/env python3
import sys, json
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    req=json.loads(line)
    print(json.dumps({"id": req.get("id",""), "ok": False}), flush=True)
`
	case fakeWorkerSlow:
		body = `#!/usr/bin/env python3
import sys, json, time
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    req=json.loads(line)
    time.sleep(2)
    print(json.dumps({"id": req.get("id",""), "ok": True, "text": "late"}), flush=True)
`
	case fakeWorkerNoise:
		body = `#!/usr/bin/env python3
import sys, json
print("not-json-noise", flush=True)
print("", flush=True)
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    req=json.loads(line)
    print("still-noise", flush=True)
    print(json.dumps({"id": req.get("id",""), "ok": True, "text": "ok-after-noise"}), flush=True)
`
	case fakeWorkerStaleID:
		body = `#!/usr/bin/env python3
import sys, json
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    req=json.loads(line)
    print(json.dumps({"id": "not-this-id", "ok": True, "text": "stale"}), flush=True)
    print(json.dumps({"id": req.get("id",""), "ok": True, "text": "matched"}), flush=True)
`
	case fakeWorkerExitOnReq:
		body = `#!/usr/bin/env python3
import sys, json
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    sys.exit(0)
`
	case fakeWorkerOnceThenExit:
		body = `#!/usr/bin/env python3
import sys, json
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    req=json.loads(line)
    print(json.dumps({"id": req.get("id",""), "ok": True, "text": "once"}), flush=True)
    sys.exit(0)
`
	case fakeWorkerNeverReady:
		body = `#!/usr/bin/env python3
import sys, time
while True:
    print("still loading", flush=True)
    time.sleep(0.05)
`
	case fakeWorkerExitImmediate:
		body = `#!/usr/bin/env python3
import sys
sys.stderr.write("boom on start\n")
sys.exit(1)
`
	default:
		body = `#!/usr/bin/env python3
import sys, json
print(json.dumps({"ready": True}), flush=True)
for line in sys.stdin:
    line=line.strip()
    if not line: continue
    req=json.loads(line)
    print(json.dumps({"id": req.get("id",""), "ok": True, "text": "病院に行った。"}), flush=True)
`
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = runtime.GOOS
	return path
}

// Minimal valid PNG (1x1).
func png1x1() []byte {
	// Precomputed 1x1 transparent PNG
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
		0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func runeOverlap(got, want string) float64 {
	g := countRunes(got)
	w := countRunes(want)
	if len(w) == 0 {
		if len(g) == 0 {
			return 1
		}
		return 0
	}
	inter := 0
	for r, wn := range w {
		gn := g[r]
		if gn < wn {
			inter += gn
		} else {
			inter += wn
		}
	}
	total := 0
	for _, n := range w {
		total += n
	}
	return float64(inter) / float64(total)
}

func countRunes(s string) map[rune]int {
	m := map[rune]int{}
	for _, r := range s {
		if r == ' ' || r == '\n' || r == '\t' || r == '\u3000' {
			continue
		}
		m[r]++
	}
	return m
}
