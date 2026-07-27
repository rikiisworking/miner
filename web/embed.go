// Package web holds server-rendered templates and static assets (HTMX, etc.).
//
// Default delivery is the embedded FS (single binary, cwd-independent).
// Dev override: set MINER_WEB_ROOT to a directory that contains templates/ and static/.
package web

import (
	"embed"
	"io/fs"
)

//go:embed templates static
var content embed.FS

// FS returns the embedded web root (templates/ and static/).
func FS() fs.FS {
	return content
}
