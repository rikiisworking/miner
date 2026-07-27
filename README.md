# miner

Local home-server app for mining Japanese novel vocabulary from a phone browser (LAN).  
Plans live under [`.scratch/novel-miner/`](.scratch/novel-miner/spec.md).

## Stack

- **Go** + **Fiber** (backend)
- **HTMX** (server-rendered HTML, embedded under `web/`)
- Application facade: `internal/app` (**MiningApp**); HTTP is a thin adapter

Domain + seam vocabulary: [`CONTEXT.md`](CONTEXT.md).

## Tickets

| # | Branch theme | Status |
|---|--------------|--------|
| 01 | LAN shell + PIN gate | done |
| 02 | Sentence analyze (paste): furigana + content words | this branch |

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

- **L1:** `internal/app` — MiningApp unlock + `AnalyzeSentence` with fake ports  
- **L2:** `internal/httpapi` — Fiber `app.Test` session / analyze HTMX fragment  
- **L3:** `e2e` — headless browser (rod): PIN → paste → analyze (ruby + content list)

L3 uses headless Chromium via [rod](https://go-rod.github.io/) (downloads browser once into `~/.cache/rod`).  
HTMX is **vendored** at `web/static/htmx.min.js` (no CDN) so UI tests do not hang on external network.

Analyzer is a **port** (`JapaneseAnalyzer`). Production wires `internal/adapters/analyzer.Stub` (fixture sentences + whole-text fallback) until a real local morphological engine is chosen. Content vs non-content uses an explicit flag on tokens; real adapters map POS tags into that flag.

### Analyze (ticket 02)

After PIN unlock: paste/type Japanese text → **Analyze** → HTMX swaps HTML with:

- sentence as **HTML ruby** furigana  
- **content-word** list (surface + reading; particles/function words omitted)

`POST /analyze` requires session. Force-error hook for demos/tests: paste `__analyze_error__`.

## Layout

```
cmd/miner/           # process entry
internal/app/        # MiningApp facade (test seam)
internal/ports/      # PinAuth, JapaneseAnalyzer, …
internal/adapters/   # pinauth, analyzer (stub), later OCR/store
internal/httpapi/    # Fiber + templates
web/templates/       # HTML + HTMX (shell, pin, analyze_result)
e2e/                 # UI click smoke
.scratch/novel-miner # product plans / tickets
```
