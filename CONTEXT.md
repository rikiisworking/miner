# CONTEXT — novel miner

Short domain + architecture glossary for this repo. Product plans live under `.scratch/novel-miner/`; this file is the **working vocabulary** for code and agents.

## Product (one line)

Home-PC web app: phone on LAN unlocks with a shared PIN, mines Japanese novel sentences (analyze → mark unknowns → queue → Markdown export). No Anki, no translation, no cloud OCR.

## Domain terms

| Term | Meaning |
|------|---------|
| **MiningApp** | Application facade for all product use-cases. **Primary test seam** (L1). |
| **PinAuth** | Port: verify shared PIN. |
| **OcrEngine** | Port: `Recognize(ctx, image) → text` (local only). Ticket 06. Prod: `adapters/ocr.NDL` (NDLOCR-Lite Python worker; engine text only, no product normalize). Test: `adapters/ocr.Static`. Product page-text hygiene: **NormalizePageText** on MiningApp before SplitSentences. Attribution: NDLOCR-Lite / NDL Lab, CC BY 4.0. |
| **NormalizePageText** | Pure helper: inter-CJK space strip + blank-line collapse. Used by IngestPage and ProposeSentences. |
| **JapaneseAnalyzer** | Port: sentence → tokens (surface, reading, content vs not). Ticket 02. Prod: `adapters/analyzer.Kagome` (kagome + MeCab-IPADIC, pure Go). Test: `adapters/analyzer.Stub`. |
| **QueueStore** | Port: durable queue entries. File JSON + Mem adapters. Surface: **Create**, **List** (unordered), **AppendUnknown** → `AppendResult`, **ClearAll**. Product display/export order lives on **MiningApp.ListQueue** / **ExportMarkdown** (FirstUnknownAt, then id). Sentinels: `queuestore.ErrEmptyID`, `ErrDuplicateID`. |
| **IngestPage** | MiningApp: image → OCR (ctx, **MaxIngestDuration**) → **NormalizePageText** → candidates. Empty image → **ErrEmptyImage**; empty OCR text → **ErrEmptyPage**. Single-flight **ErrIngestBusy**; cancel **ErrIngestCanceled**. Never writes queue. |
| **Queue entry** | Stable id + sentence text + ordered unique unknowns + first-unknown-at. New mining pass ⇒ new id (no merge-by-text). |
| **Analyze pass / PassID** | Ephemeral id returned by each `AnalyzeSentence`. First `AddUnknown` with that pass creates the entry; later unknowns (or concurrent first-taps) with the same pass append. Not durable; not the queue entry id. See **Pass protocol**. |
| **Unknown** | Surface form tapped from content-word list; stored as shown (not lemma). |
| **Content word** | Token shown in list (nouns/verbs/adjectives/…); drop particles/aux/symbols. |
| **Export document** | UTF-8 Markdown nested list; order by first-unknown-at. **Does not clear queue.** `GET /export`. |
| **Clear all** | Separate control; confirm when N≥1; only way to wipe queue in v1. `POST /queue/clear`. |
| **Page text / candidates** | Multi-sentence paste → `ProposeSentences` / `SplitSentences` → pick one as working sentence → analyze. Ephemeral; no queue write until mark unknown. `POST /page-text`. Empty → **ErrEmptyPage**. |
| **SplitSentences** | Pure helper: split on `。！？` (+ fullwidth `．`, halfwidth `!?`). No terminator → one blob. Empty → empty list. |
| **MaxUploadBytes** | Product cap for photo ingest: 10 MiB. Owned by **MiningApp** (`app.MaxUploadBytes`). HTTP BodyLimit and ocrtest assert against it. |

Avoid: Card, SM-2 Review, lemma identity, Article/RSS Source (other products).

## Pass protocol

One analyze result binds concurrent/subsequent first-taps to **one** queue entry.

| Field | Where | Role |
|-------|--------|------|
| `pass_id` | Analyze response (hidden); each mark form `hx-include`s it | Ephemeral bind key. First `AddUnknown` with this pass creates entry; server maps pass→entry. Cleared on process restart and on Clear all. |
| `entry_id` | Empty until first save; OOB-swapped into `#entry_id` after save | Durable queue entry id for further appends on that entry. |
| `sentence` + `surface` | Each mark form | Working sentence text + surface form to save. |

Rules: never merge by sentence text alone; same `pass_id` → same entry; new analyze → new `pass_id` → new entry. Product rule lives in **MiningApp**; HTMX only carries the three fields.

## Stack (frozen)

- **Go** + **Fiber** (HTTP adapter)
- **HTMX** + server HTML (`web/` embedded; optional `MINER_WEB_ROOT` for disk override)
- Single process home server; session cookie **HttpOnly** + **SameSite=Lax**. In-memory sessions die on process restart; cookie max-age is long (home always-on box) so re-PIN is not forced mid-session while the process lives
- Durable queue file (`MINER_DATA_DIR/queue.json`, default `data/`, owner-only `0o600`); survives restart. Session does not
- Unlock rate-limited per client IP (HTTP adapter) to slow LAN PIN guessing

## Seams

1. **MiningApp** — product rules and L1 tests. HTTP must not re-implement business rules.
2. **Ports** (`internal/ports`) — PinAuth + JapaneseAnalyzer + QueueStore + OcrEngine. Adapters under `internal/adapters/` (pinauth, analyzer Kagome / Stub, ocr NDL / Static, queuestore file + mem for tests).
3. **httpapi** — Fiber, cookies, templates, static files. Thin map: request → MiningApp → HTML/file. BodyLimit = MaxUploadBytes + multipart margin. HTMX partials for page-text / photo-ingest candidates (`POST /ingest`), analyze, unknown feedback; full pages for shell/queue. Session gate deny for HTMX uses generic `htmx_error` fragment (never a feature partial).
4. **web.FS()** — templates + static assets (embed by default).

## Testing layers

| Layer | Where | What |
|-------|--------|------|
| L1 | `internal/app` (+ pure helpers) | Product rules via MiningApp; `queuestore.Mem` / file store + `pinauth.Static` + default `ocr.Static` |
| L2 | `internal/httpapi` | Fiber `app.Test`; session/HTML; pass_id transport; default `ocr.Static` |
| L3 | `e2e` | Headless browser clicks; local assets; default `ocr.Static` |
| OCR-real | selected tests only | `ocr.MustEngine` (host NDLOCR-Lite + `MINER_NDL_*`); **skips** if missing |

Command: `make test` (full suite). Shared test doubles: `queuestore.NewMem()`, `pinauth.Static`, `ocr.Static` (not package-local copies). Real NDLOCR-Lite only for tests that intentionally prove the worker.

## Architectural rule (C1)

**New use-cases land on MiningApp first.** Fiber handlers only adapt transport. If a rule can be unit-tested without HTTP, it belongs in MiningApp (or a port behind it), not in a handler body.
