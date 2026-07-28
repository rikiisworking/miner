package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
	"github.com/rikiisworking/miner/internal/adapters/ocr"
	"github.com/rikiisworking/miner/internal/adapters/pinauth"
	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/httpapi"
	"github.com/rikiisworking/miner/web"
)

func main() {
	pin := os.Getenv("MINER_PIN")
	if pin == "" {
		log.Fatal("MINER_PIN is required (shared PIN; not committed to source)")
	}

	addr := os.Getenv("MINER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	dataDir := os.Getenv("MINER_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	queuePath := filepath.Join(dataDir, "queue.json")

	webFS, err := resolveWebFS()
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}

	ocrEngine, err := ocr.NewTesseractFromEnv()
	if err != nil {
		log.Fatalf("OCR engine: %v\nInstall local tesseract with Japanese data, e.g.:\n  sudo apt install tesseract-ocr tesseract-ocr-jpn tesseract-ocr-jpn-vert\nOr set MINER_TESSERACT and MINER_TESSDATA_PREFIX to a user-local install.", err)
	}

	mining := app.NewMiningApp(
		pinauth.Static{Secret: pin},
		analyzer.Stub{},
		queuestore.NewFile(queuePath),
		ocrEngine,
	)
	srv, err := httpapi.New(httpapi.Config{
		MiningApp: mining,
		WebFS:     webFS,
		Addr:      addr,
	})
	if err != nil {
		log.Fatalf("http server: %v", err)
	}

	logLANHints(addr)
	log.Printf("miner listening on %s (queue=%s)", addr, queuePath)
	if err := srv.Listen(); err != nil {
		log.Fatal(err)
	}
}

// resolveWebFS prefers embedded assets; MINER_WEB_ROOT forces on-disk templates/static for dev.
func resolveWebFS() (fs.FS, error) {
	if root := os.Getenv("MINER_WEB_ROOT"); root != "" {
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("MINER_WEB_ROOT=%q: %w", root, err)
		}
		return os.DirFS(root), nil
	}
	return web.FS(), nil
}
