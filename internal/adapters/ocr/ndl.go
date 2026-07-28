package ocr

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rikiisworking/miner/internal/ports"
)

// Sentinel errors for the NDLOCR-Lite adapter.
var (
	ErrNDLNotFound    = errors.New("ocr: NDLOCR-Lite worker or python not found")
	ErrWorkerNotReady = errors.New("ocr: NDLOCR-Lite worker not ready")
)

const (
	defaultReadyTimeout = 120 * time.Second
	defaultWorkerName   = "ndl_ocr_worker.py"
)

// NDL is a local OcrEngine that talks to a long-lived NDLOCR-Lite Python worker.
// No CGO. Requires a ndlocr-lite install (MINER_NDL_ROOT) and Python with deps.
type NDL struct {
	// Python is the interpreter path. Empty → "python3".
	Python string
	// Worker is the path to scripts/ndl_ocr_worker.py.
	Worker string
	// Root is the ndlocr-lite clone (MINER_NDL_ROOT). Required.
	Root string
	// Device is cpu or cuda (passed via env to worker).
	Device string
	// EnableTCY enables 縦中横 helper (MINER_NDL_ENABLE_TCY).
	EnableTCY bool
	// ReadyTimeout bounds wait for {"ready":true}. 0 → defaultReadyTimeout.
	ReadyTimeout time.Duration

	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   *bufio.Scanner
	stderr   bytesBuffer
	ready    bool
	reqSeq   atomic.Uint64
	started  bool
	startErr error
}

// thin buffer holder so tests can inspect stderr without importing bytes everywhere.
type bytesBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *bytesBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// NDLConfig is the production constructor input.
type NDLConfig struct {
	Python       string
	Worker       string
	Root         string
	Device       string
	EnableTCY    bool
	ReadyTimeout time.Duration
}

// NewNDL builds an NDL engine and starts the worker (loads models).
// Fails if python/worker/root are missing or ready never arrives.
func NewNDL(cfg NDLConfig) (*NDL, error) {
	n := &NDL{
		Python:       strings.TrimSpace(cfg.Python),
		Worker:       strings.TrimSpace(cfg.Worker),
		Root:         strings.TrimSpace(cfg.Root),
		Device:       strings.TrimSpace(cfg.Device),
		EnableTCY:    cfg.EnableTCY,
		ReadyTimeout: cfg.ReadyTimeout,
	}
	if n.Python == "" {
		n.Python = "python3"
	}
	if n.Device == "" {
		n.Device = "cpu"
	}
	if n.ReadyTimeout <= 0 {
		n.ReadyTimeout = defaultReadyTimeout
	}
	if err := n.validatePaths(); err != nil {
		return nil, err
	}
	if err := n.ensureWorker(); err != nil {
		return nil, err
	}
	return n, nil
}

// NewNDLFromEnv reads MINER_NDL_* and starts the worker.
func NewNDLFromEnv() (*NDL, error) {
	return NewNDL(NDLConfig{
		Python:    os.Getenv("MINER_NDL_PYTHON"),
		Worker:    os.Getenv("MINER_NDL_WORKER"),
		Root:      os.Getenv("MINER_NDL_ROOT"),
		Device:    os.Getenv("MINER_NDL_DEVICE"),
		EnableTCY: envTruthy("MINER_NDL_ENABLE_TCY"),
	})
}

func envTruthy(k string) bool {
	v := strings.TrimSpace(os.Getenv(k))
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func (n *NDL) validatePaths() error {
	if n.Root == "" {
		return fmt.Errorf("%w: set MINER_NDL_ROOT to ndlocr-lite clone", ErrNDLNotFound)
	}
	root, err := filepath.Abs(n.Root)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNDLNotFound, err)
	}
	n.Root = root
	if st, err := os.Stat(filepath.Join(n.Root, "src", "ocr.py")); err != nil || st.IsDir() {
		return fmt.Errorf("%w: MINER_NDL_ROOT=%s missing src/ocr.py", ErrNDLNotFound, n.Root)
	}

	if n.Worker == "" {
		n.Worker = findDefaultWorker()
	}
	if n.Worker == "" {
		return fmt.Errorf("%w: set MINER_NDL_WORKER to scripts/ndl_ocr_worker.py", ErrNDLNotFound)
	}
	w, err := filepath.Abs(n.Worker)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNDLNotFound, err)
	}
	n.Worker = w
	if st, err := os.Stat(n.Worker); err != nil || st.IsDir() {
		return fmt.Errorf("%w: worker %s", ErrNDLNotFound, n.Worker)
	}

	// Resolve python.
	if filepath.IsAbs(n.Python) || strings.Contains(n.Python, string(os.PathSeparator)) {
		if st, err := os.Stat(n.Python); err != nil || st.IsDir() {
			return fmt.Errorf("%w: python %s", ErrNDLNotFound, n.Python)
		}
	} else {
		p, err := exec.LookPath(n.Python)
		if err != nil {
			return fmt.Errorf("%w: %s not on PATH", ErrNDLNotFound, n.Python)
		}
		n.Python = p
	}
	return nil
}

func findDefaultWorker() string {
	candidates := []string{
		filepath.Join("scripts", defaultWorkerName),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "scripts", defaultWorkerName),
			filepath.Join(dir, "..", "scripts", defaultWorkerName),
		)
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "scripts", defaultWorkerName))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

type workerReq struct {
	ID        string `json:"id"`
	ImagePath string `json:"image_path"`
}

type workerResp struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Text  string `json:"text,omitempty"`
	Error string `json:"error,omitempty"`
	Ready bool   `json:"ready,omitempty"`
}

func (n *NDL) ensureWorker() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.ready && n.cmd != nil && n.cmd.Process != nil {
		return nil
	}
	if n.started && n.startErr != nil && n.cmd == nil {
		// previous hard failure — allow retry by clearing
	}
	return n.startLocked()
}

func (n *NDL) startLocked() error {
	n.stopLocked()
	n.started = true
	n.startErr = nil
	n.ready = false
	n.stderr = bytesBuffer{}

	cmd := exec.Command(n.Python, n.Worker)
	cmd.Env = n.workerEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		n.startErr = err
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		n.startErr = err
		return err
	}
	cmd.Stderr = &n.stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		n.startErr = fmt.Errorf("%w: start worker: %v", ErrNDLNotFound, err)
		return n.startErr
	}
	n.cmd = cmd
	n.stdin = stdin
	n.stdout = bufio.NewScanner(stdout)
	// OCR pages can be large text; raise scanner buffer.
	n.stdout.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	deadline := time.Now().Add(n.ReadyTimeout)
	for {
		if time.Now().After(deadline) {
			n.stopLocked()
			n.startErr = fmt.Errorf("%w: timeout waiting for ready (stderr=%s)", ErrWorkerNotReady, truncate(n.stderr.String(), 400))
			return n.startErr
		}
		if !n.stdout.Scan() {
			err := n.stdout.Err()
			n.stopLocked()
			if err == nil {
				err = io.EOF
			}
			n.startErr = fmt.Errorf("%w: worker exited before ready: %v (stderr=%s)", ErrWorkerNotReady, err, truncate(n.stderr.String(), 400))
			return n.startErr
		}
		line := strings.TrimSpace(n.stdout.Text())
		if line == "" {
			continue
		}
		var resp workerResp
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			// ignore non-JSON noise
			continue
		}
		if resp.Ready {
			n.ready = true
			return nil
		}
	}
}

func (n *NDL) workerEnv() []string {
	env := os.Environ()
	env = append(env, "MINER_NDL_ROOT="+n.Root)
	env = append(env, "MINER_NDL_DEVICE="+n.Device)
	if n.EnableTCY {
		env = append(env, "MINER_NDL_ENABLE_TCY=1")
	} else {
		env = append(env, "MINER_NDL_ENABLE_TCY=0")
	}
	// Quiet numpy/onnx logs a bit
	env = append(env, "OMP_NUM_THREADS=1")
	return env
}

func (n *NDL) stopLocked() {
	if n.stdin != nil {
		_ = n.stdin.Close()
		n.stdin = nil
	}
	if n.cmd != nil && n.cmd.Process != nil {
		_ = n.cmd.Process.Kill()
		_, _ = n.cmd.Process.Wait()
	}
	n.cmd = nil
	n.stdout = nil
	n.ready = false
}

// Close stops the worker process.
func (n *NDL) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stopLocked()
	return nil
}

// Recognize implements ports.OcrEngine.
// Returns engine text with trailing whitespace trimmed only — product
// page-text normalize lives in MiningApp.
func (n *NDL) Recognize(ctx context.Context, image []byte) (string, error) {
	if len(image) == 0 {
		return "", ErrEmptyImage
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "miner-ocr-*"+imageExt(image))
	if err != nil {
		return "", fmt.Errorf("ocr: temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(image); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("ocr: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("ocr: close temp: %w", err)
	}

	type result struct {
		text string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		text, err := n.recognizePath(tmpPath)
		ch <- result{text, err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		return r.text, r.err
	}
}

func (n *NDL) recognizePath(imagePath string) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.ready {
		if err := n.startLocked(); err != nil {
			return "", err
		}
	}

	id := fmt.Sprintf("%d", n.reqSeq.Add(1))
	req := workerReq{ID: id, ImagePath: imagePath}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	if _, err := n.stdin.Write(append(payload, '\n')); err != nil {
		// try one restart
		if err2 := n.startLocked(); err2 != nil {
			return "", fmt.Errorf("%w: write to worker: %v; restart: %v", ErrRecognizeFailed, err, err2)
		}
		if _, err := n.stdin.Write(append(payload, '\n')); err != nil {
			return "", fmt.Errorf("%w: write to worker: %v", ErrRecognizeFailed, err)
		}
	}

	for {
		if !n.stdout.Scan() {
			err := n.stdout.Err()
			n.stopLocked()
			if err == nil {
				err = io.EOF
			}
			return "", fmt.Errorf("%w: worker closed: %v (stderr=%s)", ErrRecognizeFailed, err, truncate(n.stderr.String(), 400))
		}
		line := strings.TrimSpace(n.stdout.Text())
		if line == "" {
			continue
		}
		var resp workerResp
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		if resp.Ready {
			continue
		}
		if resp.ID != "" && resp.ID != id {
			// stale / mismatch — keep reading
			continue
		}
		if !resp.OK {
			msg := resp.Error
			if msg == "" {
				msg = "unknown worker error"
			}
			return "", fmt.Errorf("%w: %s", ErrRecognizeFailed, msg)
		}
		return strings.TrimSpace(resp.Text), nil
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

var _ ports.OcrEngine = (*NDL)(nil)
