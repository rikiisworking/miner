package main

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
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

	mining := app.NewMiningApp(
		pinauth.Static{Secret: pin},
		analyzer.Stub{},
		queuestore.NewFile(queuePath),
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

func logLANHints(addr string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		// addr might be ":8080"
		if strings.HasPrefix(addr, ":") {
			port = strings.TrimPrefix(addr, ":")
			host = ""
		} else {
			return
		}
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		fmt.Fprintf(os.Stderr, "LAN: open http://<this-pc-ip>:%s from your phone on the same Wi‑Fi\n", port)
		ifaces, _ := net.Interfaces()
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, _ := iface.Addrs()
			for _, a := range addrs {
				var ip net.IP
				switch v := a.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() {
					continue
				}
				if v4 := ip.To4(); v4 != nil {
					fmt.Fprintf(os.Stderr, "  try http://%s:%s\n", v4.String(), port)
				}
			}
		}
		fmt.Fprintf(os.Stderr, "Dev: http://127.0.0.1:%s\n", port)
	}
}
