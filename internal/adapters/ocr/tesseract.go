package ocr

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rikiisworking/miner/internal/ports"
)

// Default OCR settings for Japanese novel pages (horizontal + vertical models).
const (
	DefaultTesseractLang = "jpn+jpn_vert"
	DefaultTesseractPSM  = 3 // fully automatic page segmentation
	defaultOCRTimeout    = 60 * time.Second
)

// Sentinel errors for the Tesseract adapter.
var (
	ErrTesseractNotFound = errors.New("ocr: tesseract binary not found")
	ErrEmptyImage        = errors.New("ocr: empty image")
	ErrRecognizeFailed   = errors.New("ocr: tesseract failed")
)

// Tesseract is a local OcrEngine that shells out to the tesseract CLI.
// No CGO. Requires tesseract + Japanese traineddata (jpn / jpn_vert) on the host.
// Prefer NewTesseract / NewTesseractFromEnv so defaults and resolved bin are set once.
type Tesseract struct {
	// Bin is the tesseract executable path. Empty → look up "tesseract" on PATH
	// (or MINER_TESSERACT via NewTesseractFromEnv).
	Bin string
	// Lang is the -l value (e.g. "jpn", "jpn+jpn_vert"). Default DefaultTesseractLang.
	Lang string
	// TessdataPrefix sets TESSDATA_PREFIX when non-empty.
	TessdataPrefix string
	// PSM is --psm mode. 0 means use DefaultTesseractPSM.
	PSM int
	// Timeout bounds one Recognize call. 0 → defaultOCRTimeout.
	Timeout time.Duration

	// resolvedBin is set by NewTesseract after LookPath (avoids per-Recognize resolve).
	resolvedBin string
}

// TesseractConfig is the production constructor input.
type TesseractConfig struct {
	Bin            string
	Lang           string
	TessdataPrefix string
	PSM            int
	Timeout        time.Duration
}

// NewTesseract builds a Tesseract engine and verifies the binary is resolvable.
// Does not require a successful OCR call (langs checked on first Recognize if missing).
func NewTesseract(cfg TesseractConfig) (*Tesseract, error) {
	t := &Tesseract{
		Bin:            strings.TrimSpace(cfg.Bin),
		Lang:           strings.TrimSpace(cfg.Lang),
		TessdataPrefix: strings.TrimSpace(cfg.TessdataPrefix),
		PSM:            cfg.PSM,
		Timeout:        cfg.Timeout,
	}
	if t.Lang == "" {
		t.Lang = DefaultTesseractLang
	}
	if t.PSM == 0 {
		t.PSM = DefaultTesseractPSM
	}
	if t.Timeout <= 0 {
		t.Timeout = defaultOCRTimeout
	}
	bin, err := t.resolveBin()
	if err != nil {
		return nil, err
	}
	t.resolvedBin = bin
	return t, nil
}

// NewTesseractFromEnv reads MINER_TESSERACT, MINER_OCR_LANG, MINER_TESSDATA_PREFIX.
func NewTesseractFromEnv() (*Tesseract, error) {
	return NewTesseract(TesseractConfig{
		Bin:            os.Getenv("MINER_TESSERACT"),
		Lang:           os.Getenv("MINER_OCR_LANG"),
		TessdataPrefix: os.Getenv("MINER_TESSDATA_PREFIX"),
	})
}

func (t *Tesseract) resolveBin() (string, error) {
	bin := strings.TrimSpace(t.Bin)
	if bin == "" {
		p, err := exec.LookPath("tesseract")
		if err != nil {
			return "", fmt.Errorf("%w: install tesseract-ocr with Japanese data (jpn, jpn_vert), or set MINER_TESSERACT", ErrTesseractNotFound)
		}
		return p, nil
	}
	// Absolute or relative path given.
	if filepath.IsAbs(bin) || strings.Contains(bin, string(os.PathSeparator)) {
		if st, err := os.Stat(bin); err != nil || st.IsDir() {
			return "", fmt.Errorf("%w: %s", ErrTesseractNotFound, bin)
		}
		return bin, nil
	}
	p, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrTesseractNotFound, bin)
	}
	return p, nil
}

// Recognize implements ports.OcrEngine.
// Returns engine stdout with trailing whitespace trimmed only — product
// page-text normalize (inter-CJK strip, blank lines) is MiningApp's job.
func (t *Tesseract) Recognize(ctx context.Context, image []byte) (string, error) {
	if len(image) == 0 {
		return "", ErrEmptyImage
	}
	if ctx == nil {
		ctx = context.Background()
	}

	bin := t.resolvedBin
	if bin == "" {
		var err error
		bin, err = t.resolveBin()
		if err != nil {
			return "", err
		}
		t.resolvedBin = bin
	}

	// Defaults applied in NewTesseract; re-apply only if zero-value struct used.
	lang := t.Lang
	if lang == "" {
		lang = DefaultTesseractLang
	}
	psm := t.PSM
	if psm == 0 {
		psm = DefaultTesseractPSM
	}
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = defaultOCRTimeout
	}

	ext := imageExt(image)
	tmp, err := os.CreateTemp("", "miner-ocr-*"+ext)
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

	// stdout path: "stdout" tells tesseract to write text to stdout.
	args := []string{
		tmpPath,
		"stdout",
		"-l", lang,
		"--psm", fmt.Sprintf("%d", psm),
	}

	env := os.Environ()
	if t.TessdataPrefix != "" {
		env = append(env, "TESSDATA_PREFIX="+t.TessdataPrefix)
	}

	// Nest adapter timeout under caller ctx (MiningApp MaxIngestDuration / HTTP cancel).
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, bin, args...)
	cmd.Env = env
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if err := runCtx.Err(); err != nil {
			return "", err
		}
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%w: %s", ErrRecognizeFailed, msg)
	}

	return strings.TrimSpace(outBuf.String()), nil
}

// imageExt picks a temp-file suffix from magic bytes so leptonica decodes correctly.
func imageExt(image []byte) string {
	if len(image) >= 8 && string(image[:8]) == "\x89PNG\r\n\x1a\n" {
		return ".png"
	}
	if len(image) >= 3 && image[0] == 0xff && image[1] == 0xd8 && image[2] == 0xff {
		return ".jpg"
	}
	if len(image) >= 6 && (string(image[:6]) == "GIF87a" || string(image[:6]) == "GIF89a") {
		return ".gif"
	}
	if len(image) >= 2 && string(image[:2]) == "BM" {
		return ".bmp"
	}
	if len(image) >= 12 && string(image[:4]) == "RIFF" && string(image[8:12]) == "WEBP" {
		return ".webp"
	}
	// Unknown: still try; leptonica may sniff. Prefer .img over empty.
	return ".img"
}

// Ensure Tesseract satisfies the port at compile time.
var _ ports.OcrEngine = (*Tesseract)(nil)
