# miner

Local home-server app for mining Japanese novel vocabulary from a phone browser on your LAN.  
Unlock with a shared PIN → capture or paste a page → analyze → mark unknowns → export Markdown.  
No Anki, no cloud OCR, no translation API — everything stays on your PC.

Product plans: [`.scratch/novel-miner/`](.scratch/novel-miner/spec.md) · Domain vocabulary: [`CONTEXT.md`](CONTEXT.md)  
License: [MIT](LICENSE)

---

## Table of contents

1. [What it is](#what-it-is)
2. [Architecture at a glance](#architecture-at-a-glance)
3. [Learner flow](#learner-flow)
4. [Pass protocol (why one tap doesn’t make two cards)](#pass-protocol-why-one-tap-doesnt-make-two-cards)
5. [Repository layout](#repository-layout)
6. [HTTP routes](#http-routes)
7. [Quick start (PC)](#quick-start-pc)
8. [Stepped setup](#stepped-setup)
9. [Phone on LAN (step-by-step)](#phone-on-lan-step-by-step)
10. [Configuration](#configuration)
11. [OCR (NDLOCR-Lite)](#ocr-ndlocr-lite)
12. [Daily use walkthrough](#daily-use-walkthrough)
13. [Testing](#testing)
14. [Tickets](#tickets)
15. [Feature notes](#feature-notes)

---

## What it is

| | |
|--|--|
| **Runs on** | Your home PC (single Go process) |
| **Used from** | Phone browser on the **same Wi‑Fi** (or localhost on the PC) |
| **Auth** | One shared PIN (session cookie; re-PIN after process restart) |
| **Durable data** | Queue file only (`MINER_DATA_DIR/queue.json`) |
| **Ephemeral** | Session, analyze `pass_id` map, OCR image bytes |

```mermaid
flowchart LR
  subgraph Phone
    B[Browser HTMX]
  end
  subgraph PC
    H[httpapi Fiber]
    M[MiningApp]
    Q[(queue.json)]
    T[NDLOCR-Lite worker]
  end
  B -->|LAN HTTP| H
  H --> M
  M --> Q
  M --> T
```

---

## Architecture at a glance

Product rules live in **MiningApp**. Fiber handlers only map HTTP ↔ facade. Ports keep adapters swappable.

```mermaid
flowchart TB
  subgraph HTTP["internal/httpapi"]
    F[Fiber routes + session]
    TPL[templates / static]
  end
  subgraph APP["internal/app — MiningApp L1 seam"]
    UC[Unlock · IngestPage · ProposeSentences<br/>AnalyzeSentence · AddUnknown<br/>ListQueue · ExportMarkdown · ClearAll]
  end
  subgraph PORTS["internal/ports"]
    P1[PinAuth]
    P2[JapaneseAnalyzer]
    P3[QueueStore]
    P4[OcrEngine]
  end
  subgraph ADP["internal/adapters"]
    A1[pinauth.Static]
    A2[analyzer.Stub]
    A3[queuestore.File / Mem]
    A4[ocr.NDL / Static]
  end
  F --> UC
  TPL --> F
  UC --> P1 & P2 & P3 & P4
  P1 --> A1
  P2 --> A2
  P3 --> A3
  P4 --> A4
```

| Layer | Package | Responsibility |
|-------|---------|----------------|
| Process | `cmd/miner` | Env, data dir, wire adapters, listen, LAN URL hints |
| HTTP adapter | `internal/httpapi` | Cookies, multipart, HTMX partials, status ↔ product errors |
| Facade | `internal/app` | All product rules (C1: new use-cases land here first) |
| Ports | `internal/ports` | Small interfaces only |
| Adapters | `internal/adapters/*` | PIN, analyzer stub, file/mem queue, NDLOCR-Lite / Static OCR |
| Web | `web/` | Embedded templates + HTMX + camera JS |

**Architectural rule (C1):** if a rule can be unit-tested without HTTP, it belongs on **MiningApp**, not in a handler body.

### Data that survives restart

| Data | Survives restart? | Where |
|------|-------------------|--------|
| Queue entries + unknowns | Yes | `MINER_DATA_DIR/queue.json` (owner-only `0o600`) |
| Session “unlocked” | No | In-memory Fiber session store |
| `pass_id` → entry bind | No | In-memory on MiningApp |
| Uploaded / camera images | Never written | Held in request memory, discarded |

---

## Learner flow

```mermaid
flowchart TD
  PIN[1. Enter PIN] --> SHELL[2. Mine shell]
  SHELL --> IN[3. Ingest page]
  IN --> CAM[Camera capture]
  IN --> UP[Photo upload]
  IN --> PASTE[Paste page text]
  IN --> ONE[Type one sentence]
  CAM --> CAND[Sentence candidates]
  UP --> CAND
  PASTE --> CAND
  CAND --> PICK[4. Pick / edit working sentence]
  ONE --> ANALYZE
  PICK --> ANALYZE[5. Analyze → furigana + content words]
  ANALYZE --> TAP[6. Tap unknowns]
  TAP --> Q[7. Queue list]
  Q --> EX[8. Export Markdown]
  Q --> CL[9. Clear all]
```

1. **PIN** unlock → mining shell  
2. **Ingest** a page — camera, upload (≤10 MiB), multi-sentence paste, or single sentence  
3. **Pick** a candidate (or edit) → **Analyze** → HTML ruby + content-word list  
4. **Tap** content words → save unknowns (save / duplicate feedback)  
5. **Queue** → list entries → **Export Markdown** (queue unchanged) or **Clear all**  

No per-unknown remove in v1. Photos discarded after OCR. Primary material = novel prose; non-novel is best-effort.

---

## Pass protocol (why one tap doesn’t make two cards)

Each **Analyze** returns an ephemeral **`pass_id`**. First successful mark with that pass creates one durable queue entry; later marks (or concurrent multi-taps) with the same pass append to the **same** entry. Re-analyze → new pass → **new** entry even if the sentence text is identical (no merge-by-text).

```mermaid
sequenceDiagram
  participant UI as Phone UI
  participant HTTP as httpapi
  participant App as MiningApp
  participant Store as QueueStore

  UI->>HTTP: POST /analyze sentence
  HTTP->>App: AnalyzeSentence
  App-->>UI: tokens + pass_id (entry_id empty)

  par Concurrent first taps
    UI->>HTTP: POST /unknowns surface=本 pass_id=P
    UI->>HTTP: POST /unknowns surface=私 pass_id=P
  end
  HTTP->>App: AddUnknown …
  App->>Store: Create once, then AppendUnknown
  App-->>UI: same entry_id (OOB) + feedback
```

| Field | Durable? | Role |
|-------|----------|------|
| `pass_id` | No | Binds taps from one analyze result to one entry |
| `entry_id` | Yes (in queue) | Set after first save; further appends use it |
| `sentence` + `surface` | In queue after save | Working sentence + tapped surface form |

Clear all wipes the queue **and** pass bindings (coordinated so concurrent mark+clear cannot orphan binds).

---

## Repository layout

```
cmd/miner/                    # process entry, LAN hints, resolveWebFS
internal/app/                 # MiningApp facade (primary test seam)
internal/ports/               # PinAuth, JapaneseAnalyzer, QueueStore, OcrEngine
internal/adapters/pinauth/    # static shared PIN
internal/adapters/analyzer/   # Stub JapaneseAnalyzer (fixtures + fallback)
internal/adapters/ocr/        # NDL / NDLOCR-Lite (prod) + Static (tests)
scripts/ndl_ocr_worker.py     # long-lived Python worker for NDLOCR-Lite
scripts/install_ndlocr.sh     # make ocr-install (clone + venv + deps)
requirements-ocr.txt          # Python deps for the OCR worker venv
.deps/                        # local NDLOCR-Lite install (gitignored; make ocr-install)
internal/adapters/queuestore/ # file + mem QueueStore
internal/httpapi/             # Fiber + session + handlers
internal/ocrtest/             # OCR fixture loader (tests)
web/templates/                # pin, shell, queue, partials
web/static/                   # htmx.min.js, camera.js
e2e/                          # headless UI journeys (rod)
testdata/ocr/                 # synthetic page fixtures
.scratch/novel-miner/         # product plans / tickets
data/                         # runtime queue (created on run; gitignored)
```

| Path | When you change… |
|------|------------------|
| `internal/app` | Product rules, errors, pass/ingest behavior |
| `internal/httpapi` | Routes, cookies, HTML status mapping |
| `web/templates` | Phone UI chrome and partials |
| `internal/adapters/*` | How PIN / OCR / queue / analyzer are implemented |
| `e2e/` | Full click paths (PIN → export → clear) |

---

## HTTP routes

All mining routes require session except `/` and `POST /unlock`.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/` | PIN page or shell if already unlocked |
| `POST` | `/unlock` | Verify PIN → session cookie (rate-limited per IP) |
| `GET` | `/home` | Mining shell |
| `POST` | `/page-text` | Paste → sentence candidates (HTMX) |
| `POST` | `/ingest` | Multipart `image` → OCR → candidates (HTMX) |
| `POST` | `/analyze` | Sentence → furigana + content words + `pass_id` |
| `POST` | `/unknowns` | Mark unknown (`sentence`, `surface`, `entry_id?`, `pass_id?`) |
| `GET` | `/queue` | Queue HTML + export / clear controls |
| `GET` | `/export` | UTF-8 Markdown download (does **not** clear queue) |
| `POST` | `/queue/clear` | Wipe queue (UI confirms when N≥1) |
| `GET` | `/static/*` | HTMX, camera.js, assets |

---

## Quick start (PC)

**Prerequisites:** Go 1.22+ (see `go.mod`), and for photo OCR: NDLOCR-Lite + Python 3.12/3.11 (see [OCR](#ocr-ndlocr-lite)). Works on **Linux and macOS** (Intel + Apple Silicon).

```bash
git clone <repo-url> && cd miner
make ocr-install                 # NDLOCR-Lite → .deps/ (once; Linux or macOS)
export MINER_PIN='choose-a-shared-pin'
make run                         # picks up .deps OCR env automatically
# open http://127.0.0.1:8080
```

---

## Stepped setup

### 1. Install Go

```bash
go version   # need a recent Go that matches go.mod
```

### 2. Clone and enter the repo

```bash
git clone <repo-url>
cd miner
```

### 3. Install NDLOCR-Lite for photo / camera mining

Without NDLOCR-Lite the server **will not start** (production requires a real local OCR engine).

**Preferred (one command):**

```bash
make ocr-install
# → clones into .deps/ndlocr-lite, creates Python 3.12 (or 3.11/3.10) venv, installs requirements-ocr.txt
# re-run anytime; skips work if already healthy (OCR_UPDATE=1 to force refresh)
```

Needs: `git`, and either [uv](https://github.com/astral-sh/uv) (recommended) or `python3.12` / `3.11` / `3.10`.

| OS | Notes |
|----|--------|
| **Linux** | Default path; `MINER_NDL_DEVICE=cpu` (or `cuda` if onnxruntime-gpu + CUDA) |
| **macOS** | Intel + Apple Silicon; always **CPU** (`MINER_NDL_DEVICE=cpu`). Prefer `brew install python@3.12` or uv. Do not use system Python 3.14 for OCR wheels. |

`make run` then uses `.deps` automatically when `MINER_NDL_*` are unset. Print exports with `make ocr-env`.

**macOS example:**

```bash
# optional helpers
brew install go git
curl -LsSf https://astral.sh/uv/install.sh | sh   # or: brew install uv
# brew install python@3.12   # if not using uv to fetch Python

make ocr-install
export MINER_PIN='your-shared-pin'
make run
```

**Manual install** (custom path):

```bash
git clone https://github.com/ndl-lab/ndlocr-lite ~/src/ndlocr-lite
uv venv ~/src/ndlocr-lite/.venv --python 3.12
uv pip install --python ~/src/ndlocr-lite/.venv/bin/python -r requirements-ocr.txt
export MINER_NDL_ROOT=~/src/ndlocr-lite
export MINER_NDL_PYTHON=~/src/ndlocr-lite/.venv/bin/python
export MINER_NDL_WORKER=$PWD/scripts/ndl_ocr_worker.py
```

**Credit:** OCR uses [NDLOCR-Lite](https://github.com/ndl-lab/ndlocr-lite) by 国立国会図書館 NDL Lab (CC BY 4.0).

### 4. Choose a PIN (required)

```bash
export MINER_PIN='your-shared-pin'   # not committed; known to household only
```

### 5. Build and run

```bash
make run
# equivalent:
# make build && ./bin/miner
# go run ./cmd/miner
```

On success you should see logs like (first boot waits while the OCR worker loads models):

```text
Dev: http://127.0.0.1:8080
LAN: open http://<this-pc-ip>:8080 from your phone on the same Wi‑Fi
  try http://192.168.x.x:8080
miner listening on :8080 (queue=data/queue.json)
```

If startup fails with `OCR engine: …`, fix `MINER_NDL_ROOT` / `MINER_NDL_PYTHON` / `MINER_NDL_WORKER` (see [OCR](#ocr-ndlocr-lite)).

### 6. Open on the PC first

1. Browser → [http://127.0.0.1:8080](http://127.0.0.1:8080)  
2. Enter `MINER_PIN` → you should see the **Mine** shell  
3. Paste `私は本を読む。` → Analyze → confirm furigana + content words  

### 7. Optional: live template edit

```bash
export MINER_WEB_ROOT=/absolute/path/to/miner/web
make run
```

Uses on-disk `templates/` + `static/` instead of the embedded FS.

### 8. Stop

`Ctrl+C` in the terminal. Queue file under `data/` (or `MINER_DATA_DIR`) remains; session is gone → re-PIN next start.

---

## Phone on LAN (step-by-step)

Goal: phone browser talks to the PC process over Wi‑Fi.

```mermaid
flowchart LR
  Phone[Phone browser] -->|same Wi‑Fi| Router
  PC[PC :8080 miner] -->|same Wi‑Fi| Router
  Phone -->|http://PC_IP:8080| PC
```

### Step 1 — Same network

- Phone and PC on the **same Wi‑Fi** (not guest isolation if your router isolates clients).  
- Prefer 2.4/5 GHz home LAN; avoid phone cellular data.

### Step 2 — Bind all interfaces (default)

Default `MINER_ADDR=:8080` listens on **all** interfaces (phone-reachable).

**Wrong for phone** (loopback only):

```bash
export MINER_ADDR=127.0.0.1:8080   # phone cannot connect
```

**Right for phone:**

```bash
export MINER_ADDR=:8080            # default; all interfaces
# or explicit LAN IP:
export MINER_ADDR=192.168.1.10:8080
```

### Step 3 — Start miner and read LAN hints

```bash
export MINER_PIN='your-shared-pin'
make run
```

Copy a line like `try http://192.168.1.42:8080` from the log.

If no IPv4 listed:

```bash
# Linux example
ip -4 addr show
# or
hostname -I
```

Use a non-loopback address (not `127.0.0.1`).

### Step 4 — Firewall (if phone cannot connect)

Allow inbound TCP **8080** on the PC.

```bash
# Ubuntu ufw example
sudo ufw allow 8080/tcp
sudo ufw status
```

Windows: allow Go / `miner` through Windows Defender Firewall for private networks.  
macOS: System Settings → Network → Firewall → allow incoming for the binary if prompted.

### Step 5 — Open on the phone

1. Phone browser (Safari / Chrome).  
2. Address bar: `http://192.168.x.x:8080` (your PC IP from step 3).  
3. **http** not https (local LAN; cookie is `HttpOnly` + `SameSite=Lax`, not Secure).  
4. Enter the same `MINER_PIN` as the PC.  
5. You should see **Mine** / **Queue** nav.

### Step 6 — Camera permission (optional)

1. On Mine → **Open camera**.  
2. Allow camera for the site when prompted.  
3. **Capture page** → same OCR path as file upload.  
4. If denied / no camera → use **Page photo** upload or paste text.

HTTPS is not required for many phones on LAN, but some browsers are stricter about camera on plain HTTP; if camera fails, upload or paste still work.

### Step 7 — Smoke the full path on phone

1. Upload or capture a clear novel page (or paste text).  
2. Pick a sentence → Analyze → tap 1–2 content words.  
3. Open **Queue** → confirm entries.  
4. **Export Markdown** → file downloads.  
5. **Clear all** → confirm dialog when N≥1.

### Troubleshooting phone access

| Symptom | Check |
|---------|--------|
| Connection refused / timeout | PC running? Same Wi‑Fi? Firewall? Correct IP? |
| Opens then “Session required” | Cookie blocked? Retry unlock; avoid private-mode quirks |
| PIN always wrong | Same `MINER_PIN` as process env; rate limit after many fails (wait ~1 min) |
| No LAN IP in logs | PC offline / only loopback; connect Wi‑Fi or set `MINER_ADDR` |
| Camera blocked | Use upload; or try another browser; check site permissions |
| OCR empty / garbage | Better photo, fill frame, less tilt; edit sentence; paste text fallback |
| Server dies at start with “OCR engine” | `MINER_NDL_ROOT` / python / worker wrong — see [OCR](#ocr-ndlocr-lite) |
| First photo very slow | Normal: models warm on process start; later pages should be faster |

---

## Configuration

| Env | Required | Default | Meaning |
|-----|----------|---------|---------|
| `MINER_PIN` | **yes** | — | Shared unlock PIN (never commit) |
| `MINER_ADDR` | no | `:8080` | Listen address (`:8080` = all interfaces) |
| `MINER_WEB_ROOT` | no | *(embedded)* | Disk override of `templates/` + `static/` |
| `MINER_DATA_DIR` | no | `data` | Durable queue directory (`queue.json`) |
| `MINER_NDL_ROOT` | for OCR | `.deps/ndlocr-lite` via `make` | Absolute path to [ndlocr-lite](https://github.com/ndl-lab/ndlocr-lite) clone |
| `MINER_NDL_PYTHON` | for OCR | `.deps/.../.venv/bin/python` via `make` | Venv interpreter with `requirements-ocr.txt` deps |
| `MINER_NDL_WORKER` | for OCR | `scripts/ndl_ocr_worker.py` via `make` | Path to miner’s worker script |
| `MINER_NDL_DEVICE` | no | `cpu` | `cpu` or `cuda` (Linux GPU only; **macOS must use `cpu`**) |
| `MINER_NDL_ENABLE_TCY` | no | off | `1` / `true` enables 縦中横 helper in the worker |

Example full launch:

```bash
make ocr-install   # once
export MINER_PIN='household-pin'
export MINER_ADDR=:8080
export MINER_DATA_DIR="$HOME/.local/share/miner"
make run           # MINER_NDL_* default to .deps/
```

---

## OCR (NDLOCR-Lite)

Production photo / camera ingest uses **[NDLOCR-Lite](https://github.com/ndl-lab/ndlocr-lite)** (国立国会図書館 NDL Lab) — a CPU-friendly Japanese book/magazine OCR stack with layout detection, character recognition, and **reading-order** (縦書き columns right→left). **No Tesseract. No cloud OCR.**

### How miner wires it

```text
Phone photo / upload
  → POST /ingest (httpapi)
  → MiningApp.IngestPage
  → ports.OcrEngine.Recognize(ctx, image bytes)
  → adapters/ocr.NDL  (Go)
       writes temp image
       JSON line → scripts/ndl_ocr_worker.py (long-lived Python)
       ← plain text (trim only)
  → NormalizePageText → SplitSentences → candidates HTML
```

| Piece | Role |
|-------|------|
| `internal/ports.OcrEngine` | Small seam: `Recognize(ctx, image) → text` |
| `internal/adapters/ocr.NDL` | Prod adapter: starts/owns worker, honors cancel/deadline |
| `internal/adapters/ocr.Static` | Test double (default L1/L2/L3 — no Python needed) |
| `scripts/ndl_ocr_worker.py` | Loads ONNX models **once**, then answers JSON requests |
| `requirements-ocr.txt` | Python pins for the worker venv (install into NDL’s venv) |

Worker protocol (one JSON object per line):

```json
{"id":"1","image_path":"/tmp/miner-ocr-xxx.png"}
{"id":"1","ok":true,"text":"病院に行った。\n私は本を読む。"}
```

On startup the worker emits `{"ready":true}` after models load; miner waits (default up to 120s) before accepting traffic that needs OCR.

**Product hygiene stays in MiningApp:** inter-CJK space strip and blank-line collapse via `NormalizePageText`. The adapter returns engine text only.

### Install (home PC, CPU) — Linux & macOS

**Preferred:**

```bash
make ocr-install          # → .deps/ndlocr-lite + venv + deps (idempotent)
export MINER_PIN='your-shared-pin'
make run                  # uses .deps when MINER_NDL_* unset
```

Needs **git** and either [uv](https://github.com/astral-sh/uv) (recommended) or Python **3.12 / 3.11 / 3.10**. System Python 3.14 often lacks `onnxruntime` wheels.

| Helper | Action |
|--------|--------|
| `make ocr-install` | Clone + venv + `requirements-ocr.txt` into `.deps/` (gitignored) |
| `make ocr-env` | Print `export MINER_NDL_*=…` for the default/current paths |
| `OCR_UPDATE=1 make ocr-install` | `git pull` + reinstall deps |

| Platform | Runtime | Notes |
|----------|---------|--------|
| Linux x86_64 / arm64 | CPU or optional CUDA | `MINER_NDL_DEVICE=cpu` (default) or `cuda` |
| macOS Intel | CPU | `onnxruntime==1.23.2` pin; no CUDA |
| macOS Apple Silicon | CPU | same; first model load a few seconds |

**Manual** (custom location): set `MINER_NDL_ROOT` / `MINER_NDL_PYTHON` / `MINER_NDL_WORKER` yourself; see stepped setup.

**Verify worker alone** (optional, after install):

```bash
make ocr-env   # copy exports, or rely on .deps defaults
.deps/ndlocr-lite/.venv/bin/python scripts/ndl_ocr_worker.py
# expect one line: {"ready": true}
# then type (example): {"id":"1","image_path":"testdata/ocr/images/01_single_sentence.png"}
# Ctrl+D to exit
```

### Product rules (ingest)

| Rule | Behavior |
|------|----------|
| Max image size | 10 MiB (`app.MaxUploadBytes`) |
| Timeout | `MaxIngestDuration` (60s) on parent context |
| Single-flight | One ingest at a time (`409` if busy) |
| Queue on OCR fail | Untouched |
| Image persistence | Never written under data dir (temp file only, deleted after OCR) |
| Empty OCR text | `ErrEmptyPage` after normalize |
| Cancel mid-OCR | `ErrIngestCanceled` |

### Performance notes

| Phase | What to expect |
|-------|----------------|
| Process start | Worker loads ONNX models (~several seconds on CPU) |
| Warm page | Often ~1–few seconds for a simple novel crop |
| Cold first request | If worker not pre-started, first recognize pays model load |

Miner starts the worker in `NewNDLFromEnv()` at boot so the first phone shot is usually warm.

### Attribution & license

OCR models and code: **[NDLOCR-Lite](https://github.com/ndl-lab/ndlocr-lite)** by [NDL Lab](https://lab.ndl.go.jp/) / 国立国会図書館, licensed **CC BY 4.0**.  
Miner (this repo) remains **MIT**; keep NDL credit when you redistribute the OCR stack.

### OCR tests

| Kind | When | How |
|------|------|-----|
| Default suite | Always | `ocr.Static` — `make test` needs **no** Python/models |
| Fake worker | Always | Go unit tests drive a stub Python script (protocol only) |
| Real smoke | Host with `MINER_NDL_*` | `go test ./internal/adapters/ocr/ -run Smoke` |
| Contract | Optional | `make ocr-contract` or `MINER_OCR_CONTRACT=1` |

```bash
make ocr-install   # if not already installed

# smoke (skips if engine missing)
go test ./internal/adapters/ocr/ -count=1 -run 'Smoke' -timeout 3m -v

# full fixture contract (happy / vertical / blur / brightness / font / …)
make ocr-contract
```

Fixtures: `testdata/ocr/` (55 cases). Tags: **happy**, **vertical**, **novel**, **blur**, **brightness**, **font**, **thickness**, **colour**, **tilt**, etc. Soft (log-only) IDs for known-hard phone stress live in `internal/adapters/ocr/ndl_test.go`.

---

## Daily use walkthrough

| Goal | What to do |
|------|------------|
| Mine from photo | Mine → camera or **Page photo** → pick sentence → Analyze → tap words |
| Mine from paste | **Page text** → Propose → pick → Analyze → tap |
| One sentence only | Working sentence box → Analyze → tap |
| Review queue | **Queue** nav |
| Backup unknowns | **Export Markdown** (queue stays) |
| Wipe session work | **Clear all** (confirm) |
| After PC reboot | Start `make run` again → re-enter PIN → queue file still there |

Export shape (nested list, order by first-unknown-at):

```markdown
- 病院に行った。
  - 病院
  - 行った
- 私は本を読む。
  - 本
```

---

## Testing

```bash
make test          # L1 + L2 + L3 (no NDLOCR-Lite required)
make test-unit     # internal packages only
make test-e2e      # headless browser only
make ocr-contract  # real NDLOCR-Lite fixtures (needs MINER_NDL_*)
make lint          # vet + staticcheck + ineffassign + deadcode
```

| Layer | Where | What |
|-------|--------|------|
| **L1** | `internal/app` | Product rules via MiningApp (fakes + mem/file store + `ocr.Static`) |
| **L2** | `internal/httpapi` | Fiber `app.Test`: session, HTMX, multipart, pass transport |
| **L3** | `e2e` | rod + Chromium: PIN → ingest paths → mark → export → clear |
| **OCR-real** | selected | `MustEngine` / smoke + contract; skips without NDLOCR-Lite env |

L3 downloads Chromium once into `~/.cache/rod`.  
HTMX is **vendored** at `web/static/htmx.min.js` (no CDN) so UI tests do not hang offline.

---

## Tickets

| # | Theme | Status |
|---|--------|--------|
| 01 | LAN shell + PIN gate | done |
| 02 | Sentence analyze (paste): furigana + content words | done |
| 03 | Mark unknowns → durable queue | done |
| 04 | Export Markdown (+ Clear all) | done |
| 05 | Full-page text → pick sentence | done |
| 06 | Photo ingest + local OCR | done |
| 07 | Phone camera capture UX | done |
| 08 | Novel UX hardening | done (manual phone novel E2E still recommended) |

---

## Feature notes

### Analyze

- `POST /analyze` requires session.  
- Force-error hook for demos/tests: paste `__analyze_error__`.  
- Production analyzer is still **`analyzer.Stub`** (fixture sentences + whole-text fallback) until a real local morphological engine is chosen.  
- Known fixtures: `私は本を読む。`, `病院に行った。`.

### Camera (ticket 07)

- Client script: `web/static/camera.js` → same `POST /ingest` as upload.  
- No new routes or MiningApp rules.  
- Permission denied / no camera → in-page message; upload remains.

### Photo OCR (NDLOCR-Lite)

- Prod engine: `ocr.NDL` + `scripts/ndl_ocr_worker.py` (not Tesseract).  
- Tuned for Japanese printed books / 縦書き; phone photos are best-effort (tilt, blur, mixed light).  
- Paste path (`POST /page-text`) still works with no OCR install for pure text mining.  
- See [OCR (NDLOCR-Lite)](#ocr-ndlocr-lite) for install, env, and contract tests.

### Unknowns + queue

- Analyze alone does **not** write the queue.  
- Atomic `QueueStore.AppendUnknown`; concurrent same-`pass_id` → one entry.  
- Queue persists across restart; `pass_id` map does not.

### Export + Clear all

- Export does **not** clear.  
- Clear is the only full wipe; UI confirm when N≥1; disabled when empty.  
- Newlines in sentence/surface flattened so Markdown list structure stays intact.

---

## Make targets

| Target | Action |
|--------|--------|
| `make ocr-install` | Install NDLOCR-Lite into `.deps/` (clone + venv + deps) |
| `make ocr-env` | Print `MINER_NDL_*` export lines |
| `make run` | Build + run (needs `MINER_PIN`; OCR from env or `.deps`) |
| `make build` | `bin/miner` (no OCR install required) |
| `make test` | Full suite (120s timeout; `ocr.Static` only) |
| `make test-unit` | `./internal/...` |
| `make test-e2e` | `./e2e/...` |
| `make ocr-contract` | Real NDLOCR-Lite contract suites (`MINER_OCR_CONTRACT=1`) |
| `make lint` | static analysis helpers |

---

## License

[MIT](LICENSE) — Copyright (c) 2026 Riki  

Local OCR uses **[NDLOCR-Lite](https://github.com/ndl-lab/ndlocr-lite)** (国立国会図書館 NDL Lab), **CC BY 4.0**. Models and the NDLOCR-Lite tree are installed separately; they are not re-licensed by this MIT project.
