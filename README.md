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
| 03 | Mark unknowns → durable queue | done |
| 04 | Export Markdown (+ Clear all) | done (text-only path complete) |
| 05 | Full-page text → pick sentence | done |
| 06+ | Photo OCR / camera / hardening | next |

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

- **L1:** `internal/app` — MiningApp unlock / analyze / `AddUnknown` / `ListQueue` / `ExportMarkdown` / `ClearAll` (in-memory + real file store)  
- **L2:** `internal/httpapi` — Fiber `app.Test` session / analyze / unknowns / queue / export / clear  
- **L3:** `e2e` — headless browser (rod): PIN → analyze → mark unknown → queue → export → clear all

L3 uses headless Chromium via [rod](https://go-rod.github.io/) (downloads browser once into `~/.cache/rod`).  
HTMX is **vendored** at `web/static/htmx.min.js` (no CDN) so UI tests do not hang on external network.

### Learner flow (text-only path)

1. **PIN** unlock → mining shell  
2. **Paste** a Japanese sentence → **Analyze** → HTML ruby furigana + content-word list  
3. **Tap** a content-word row → save as unknown (feedback on save / duplicate)  
4. **Queue** nav → list of entries (sentence + unknowns)  
5. **Export Markdown** → download UTF-8 nested list (queue unchanged)  
6. **Clear all** → confirm → wipe queue (disabled when empty)

No per-unknown/per-entry remove in v1.

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
- Each analyze result carries an ephemeral **`pass_id`** (not the queue entry id)  
- First successful unknown for a pass creates a **new** queue entry id + `first-unknown-at`  
- Further unknowns on that entry append (first-tap order); duplicate surface ignored with feedback  
- Concurrent/fast multi-taps with the **same** `pass_id` still share **one** entry (server binds pass → entry; UI also queues HTMX with `hx-sync`)  
- Re-analyze (new `pass_id`), even with identical sentence text → **separate** entry (no merge-by-text)  
- Appends use atomic **`QueueStore.AppendUnknown`** (no lost unknowns under concurrent append)  
- Queue persists across server restart via file store; `pass_id` map does **not** (in-memory only)  

Routes (session required):

| Method | Path | Body / notes |
|--------|------|----------------|
| `POST` | `/analyze` | form: `sentence` → furigana + content words + `pass_id` |
| `POST` | `/unknowns` | form: `sentence`, `surface`, optional `entry_id`, optional `pass_id` |
| `GET` | `/queue` | HTML list of entries + Export / Clear all |
| `GET` | `/export` | UTF-8 Markdown download (`text/markdown`; queue unchanged) |
| `POST` | `/queue/clear` | wipe entire queue (UI confirms when N≥1) |

UI: **Mine** / **Queue** nav; save vs duplicate feedback under the content-word list.

### Export + Clear all (ticket 04)

Queue page controls:

- **Export Markdown** — `GET /export`; nested list ordered by `first-unknown-at` (tie-break entry id); unknowns in first-tap order; empty queue returns empty document  
- **Clear all** — `POST /queue/clear` after browser confirm (`Clear all N entries?`); disabled when empty  

Newlines inside sentence/surface text are flattened to spaces so list structure stays intact.

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
