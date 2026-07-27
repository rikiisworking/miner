# miner

Local home-server app for mining Japanese novel vocabulary from a phone browser (LAN).  
Plans live under [`.scratch/novel-miner/`](.scratch/novel-miner/spec.md).

## Stack

- **Go** + **Fiber** (backend)
- **HTMX** (server-rendered HTML, embedded under `web/`)
- Application facade: `internal/app` (**MiningApp**); HTTP is a thin adapter
- Durable queue: JSON file behind **QueueStore** (`MINER_DATA_DIR/queue.json`)

Domain + seam vocabulary: [`CONTEXT.md`](CONTEXT.md).

## Tickets

| # | Branch theme | Status |
|---|--------------|--------|
| 01 | LAN shell + PIN gate | done |
| 02 | Sentence analyze (paste): furigana + content words | done |
| 03 | Mark unknowns → durable queue | this branch |
| 04 | Export Markdown (+ Clear all) | next |

### Configure

| Env | Required | Default | Meaning |
|-----|----------|---------|---------|
| `MINER_PIN` | yes | — | Shared unlock PIN (not in source) |
| `MINER_ADDR` | no | `:8080` | Listen address (`0.0.0.0:8080` style via `:8080`) |
| `MINER_WEB_ROOT` | no | *(embedded)* | Optional disk override of `templates/` + `static/` for live HTML edit |
| `MINER_DATA_DIR` | no | `data` | Directory for durable queue file (`queue.json`; gitignored) |

### Run

```bash
export MINER_PIN='your-shared-pin'
make run
# or: go run ./cmd/miner
```

**Dev:** open http://127.0.0.1:8080  

**LAN / phone:** server logs suggested `http://<pc-ip>:8080` URLs. Use the same Wi‑Fi; set firewall if needed. Binding `:8080` listens on all interfaces.

Queue survives process restart (file under `MINER_DATA_DIR`). Session cookie does **not** — restart forces re-PIN.

### Test (L1 + L2 + L3)

```bash
make test
```

- **L1:** `internal/app` — MiningApp unlock / analyze / `AddUnknown` / `ListQueue` (in-memory + real file store)  
- **L2:** `internal/httpapi` — Fiber `app.Test` session / analyze / unknowns / queue  
- **L3:** `e2e` — headless browser (rod): PIN → analyze → mark unknown → queue (incl. duplicate feedback)

L3 uses headless Chromium via [rod](https://go-rod.github.io/) (downloads browser once into `~/.cache/rod`).  
HTMX is **vendored** at `web/static/htmx.min.js` (no CDN) so UI tests do not hang on external network.

### Learner flow (so far)

1. **PIN** unlock → mining shell  
2. **Paste** a Japanese sentence → **Analyze** → HTML ruby furigana + content-word list  
3. **Tap** a content-word row → save as unknown (feedback on save / duplicate)  
4. **Queue** nav → list of entries (sentence + unknowns)

No per-unknown/per-entry remove in v1. Export Markdown and Clear all land in ticket 04.

### Analyze (ticket 02)

After PIN unlock: paste/type Japanese text → **Analyze** → HTMX swaps HTML with:

- sentence as **HTML ruby** furigana  
- **content-word** list (surface + reading; particles/function words omitted)

`POST /analyze` requires session. Force-error hook for demos/tests: paste `__analyze_error__`.

Analyzer is a **port** (`JapaneseAnalyzer`). Production wires `internal/adapters/analyzer.Stub` (fixture sentences + whole-text fallback) until a real local morphological engine is chosen. Content vs non-content uses an explicit flag on tokens; real adapters map POS tags into that flag.

Known stub fixtures: `私は本を読む。`, `病院に行った。`.

### Unknowns + queue (ticket 03)

After analyze: tap a content-word row → immediate save as unknown (surface form as shown). Rules:

- Analyze/browse alone does **not** write the queue  
- First successful unknown for a working sentence creates a **new** entry id + `first-unknown-at`  
- Further unknowns append on that entry (first-tap order); duplicate surface ignored with feedback  
- Same sentence text mined again (new first unknown, empty `entry_id`) → **separate** entry (no merge-by-text)  
- Queue persists across server restart via **QueueStore** file adapter  

Routes (session required):

| Method | Path | Body / notes |
|--------|------|----------------|
| `POST` | `/unknowns` | form: `sentence`, `surface`, optional `entry_id`, optional `pass_id` (from analyze; binds multi-tap to one entry) |
| `GET` | `/queue` | HTML list of entries |

UI: **Mine** / **Queue** nav; save vs duplicate feedback under the content-word list.

## Layout

```
cmd/miner/                    # process entry
internal/app/                 # MiningApp facade (test seam)
internal/ports/               # PinAuth, JapaneseAnalyzer, QueueStore, …
internal/adapters/pinauth/    # static shared PIN
internal/adapters/analyzer/   # stub JapaneseAnalyzer
internal/adapters/queuestore/ # file QueueStore (JSON)
internal/httpapi/             # Fiber + templates
web/templates/                # shell, pin, analyze_result, unknown_feedback, queue
web/static/                   # vendored htmx
e2e/                          # UI click smoke
.scratch/novel-miner          # product plans / tickets
data/                         # runtime queue (created on run; gitignored)
```
