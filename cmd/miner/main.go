package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatalf("data dir: %v", err)
	}
	queuePath := filepath.Join(dataDir, "queue.json")

	webFS, err := resolveWebFS(os.Getenv("MINER_WEB_ROOT"))
	if err != nil {
		log.Fatalf("web assets: %v", err)
	}

	ocrEngine, err := ocr.NewNDLFromEnv()
	if err != nil {
		log.Fatalf("OCR engine: %v\nInstall NDLOCR-Lite (local Japanese book OCR), then set:\n  git clone https://github.com/ndl-lab/ndlocr-lite ~/src/ndlocr-lite\n  # python 3.12 venv + pip install -r requirements-ocr.txt (see repo)\n  export MINER_NDL_ROOT=~/src/ndlocr-lite\n  export MINER_NDL_PYTHON=~/src/ndlocr-lite/.venv/bin/python\n  export MINER_NDL_WORKER=$PWD/scripts/ndl_ocr_worker.py", err)
	}
	defer ocrEngine.Close()

	ja, err := analyzer.NewKagome()
	if err != nil {
		log.Fatalf("Japanese analyzer: %v", err)
	}

	mining := app.NewMiningApp(
		pinauth.Static{Secret: pin},
		ja,
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logLANHints(addr)
	log.Printf("miner listening on %s (queue=%s)", addr, queuePath)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Listen()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Fatal(err)
		}
	case <-ctx.Done():
		log.Printf("shutdown signal received")
		stop()
		if err := srv.Shutdown(); err != nil {
			log.Printf("http shutdown: %v", err)
		}
		if err := <-errCh; err != nil {
			log.Printf("listen: %v", err)
		}
	}
}

// resolveWebFS prefers embedded assets; non-empty root forces on-disk templates/static for dev.
func resolveWebFS(root string) (fs.FS, error) {
	if root != "" {
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			return nil, fmt.Errorf("MINER_WEB_ROOT=%q: %w", root, err)
		}
		return os.DirFS(root), nil
	}
	return web.FS(), nil
}
