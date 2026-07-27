# miner

Local home-server app for mining Japanese novel vocabulary from a phone browser (LAN).  
Plans live under [`.scratch/novel-miner/`](.scratch/novel-miner/spec.md).

## Stack

- **Go** + **Fiber** (backend)
- **HTMX** (server-rendered HTML, embedded under `web/`)
- Application facade: `internal/app` (**MiningApp**); HTTP is a thin adapter

Domain + seam vocabulary: [`CONTEXT.md`](CONTEXT.md).

## Ticket 01 — PIN shell

### Configure

| Env | Required | Default | Meaning |
|-----|----------|---------|---------|
| `MINER_PIN` | yes | — | Shared unlock PIN (not in source) |
| `MINER_ADDR` | no | `:8080` | Listen address (`0.0.0.0:8080` style via `:8080`) |
| `MINER_WEB_ROOT` | no | *(embedded)* | Optional disk override of `templates/` + `static/` for live HTML edit |

### Run

```bash
export MINER_PIN='your-shared-pin'
make run
# or: go run ./cmd/miner
```

**Dev:** open http://127.0.0.1:8080  

**LAN / phone:** server logs suggested `http://<pc-ip>:8080` URLs. Use the same Wi‑Fi; set firewall if needed. Binding `:8080` listens on all interfaces.

### Test (L1 + L2 + L3)

```bash
make test
```

- **L1:** `internal/app` — MiningApp unlock with fake PinAuth  
- **L2:** `internal/httpapi` — Fiber `app.Test` session / 401  
- **L3:** `e2e` — headless browser (rod) clicks PIN form  

L3 uses headless Chromium via [rod](https://go-rod.github.io/) (downloads browser once into `~/.cache/rod`).  
HTMX is **vendored** at `web/static/htmx.min.js` (no CDN) so UI tests do not hang on external network.

## Layout

```
cmd/miner/           # process entry
internal/app/        # MiningApp facade (test seam)
internal/ports/      # PinAuth, …
internal/adapters/   # pinauth, later OCR/analyzer/store
internal/httpapi/    # Fiber + templates
web/templates/       # HTML + HTMX
e2e/                 # UI click smoke
.scratch/novel-miner # product plans / tickets
```
