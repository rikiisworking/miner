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
	"unicode"

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
	if _, err := t.resolveBin(); err != nil {
		return nil, err
	}
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
func (t *Tesseract) Recognize(image []byte) (string, error) {
	if len(image) == 0 {
		return "", ErrEmptyImage
	}

	bin, err := t.resolveBin()
	if err != nil {
		return "", err
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%w: %s", ErrRecognizeFailed, msg)
	}

	return normalizeOCRText(outBuf.String()), nil
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

// normalizeOCRText cleans tesseract output for novel mining:
// strip inter-CJK spaces (common with jpn_vert), collapse blank lines, trim.
func normalizeOCRText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = stripInterCJKSpaces(s)

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// stripInterCJKSpaces drops ASCII/ideographic spaces that sit between Japanese
// characters (tesseract often emits "私 は 本" or per-glyph spaces on vertical text).
func stripInterCJKSpaces(s string) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i, r := range runes {
		if isSpaceRune(r) && i > 0 && i+1 < len(runes) {
			prev, next := runes[i-1], runes[i+1]
			if isJPGlyph(prev) && isJPGlyph(next) {
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\u3000'
}

// isJPGlyph: hiragana, katakana, CJK, common punctuation used in novel prose.
func isJPGlyph(r rune) bool {
	if unicode.In(r, unicode.Hiragana, unicode.Katakana, unicode.Han) {
		return true
	}
	switch r {
	case '。', '、', '！', '？', '．', '「', '」', '『', '』', '（', '）',
		'・', 'ー', '—', '…', '々', '〆', '〇', '〜', '～',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		'０', '１', '２', '３', '４', '５', '６', '７', '８', '９':
		return true
	}
	return false
}

// Ensure Tesseract satisfies the port at compile time.
var _ ports.OcrEngine = (*Tesseract)(nil)
