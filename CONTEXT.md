# CONTEXT — novel miner

Short domain + architecture glossary for this repo. Product plans live under `.scratch/novel-miner/`; this file is the **working vocabulary** for code and agents.

## Product (one line)

Home-PC web app: phone on LAN unlocks with a shared PIN, mines Japanese novel sentences (analyze → mark unknowns → queue → Markdown export). No Anki, no translation, no cloud OCR.

## Domain terms

| Term | Meaning |
|------|---------|
| **MiningApp** | Application facade for all product use-cases. **Primary test seam** (L1). |
| **PinAuth** | Port: verify shared PIN. |
| **OcrEngine** | Port: image bytes → plain text (local only). Later tickets. |
| **JapaneseAnalyzer** | Port: sentence → tokens (surface, reading, content vs not). Ticket 02. |
| **QueueStore** | Port: durable queue entries. File JSON adapter under `MINER_DATA_DIR`. Ticket 03. Includes atomic **AppendUnknown** (locked RMW). |
| **Queue entry** | Stable id + sentence text + ordered unique unknowns + first-unknown-at. New mining pass ⇒ new id (no merge-by-text). |
| **Analyze pass / PassID** | Ephemeral id returned by each `AnalyzeSentence`. First `AddUnknown` with that pass creates the entry; later unknowns (or concurrent first-taps) with the same pass append. Not durable; not the queue entry id. |
| **Unknown** | Surface form tapped from content-word list; stored as shown (not lemma). |
| **Content word** | Token shown in list (nouns/verbs/adjectives/…); drop particles/aux/symbols. |
| **Export document** | UTF-8 Markdown nested list; order by first-unknown-at. **Does not clear queue.** `GET /export`. |
| **Clear all** | Separate control; confirm when N≥1; only way to wipe queue in v1. `POST /queue/clear`. |

Avoid: Card, SM-2 Review, lemma identity, Article/RSS Source (other products).

## Stack (frozen)

- **Go** + **Fiber** (HTTP adapter)
- **HTMX** + server HTML (`web/` embedded; optional `MINER_WEB_ROOT` for disk override)
- Single process home server; session cookie **HttpOnly** + **SameSite=Lax** until process restart
- Durable queue file (`MINER_DATA_DIR/queue.json`, default `data/`); survives restart. Session does not.

## Seams

1. **MiningApp** — product rules and L1 tests. HTTP must not re-implement business rules.
2. **Ports** (`internal/ports`) — PinAuth + JapaneseAnalyzer + QueueStore; OCR later. Adapters under `internal/adapters/` (pinauth, analyzer stub, queuestore file).
3. **httpapi** — Fiber, cookies, templates, static files. Thin map: request → MiningApp → HTML/file. HTMX partials for analyze + unknown feedback; full pages for shell/queue. Session gate deny for HTMX uses generic `auth_error` fragment (never a feature partial).
4. **web.FS()** — templates + static assets (embed by default).

## Testing layers

| Layer | Where | What |
|-------|--------|------|
| L1 | `internal/app` (+ pure helpers) | Product rules via MiningApp; fake ports + real file store for persistence/concurrency |
| L2 | `internal/httpapi` | Fiber `app.Test`; session/HTML |
| L3 | `e2e` | Headless browser clicks; local assets only |

Command: `make test` (full suite).

## Architectural rule (C1)

**New use-cases land on MiningApp first.** Fiber handlers only adapt transport. If a rule can be unit-tested without HTTP, it belongs in MiningApp (or a port behind it), not in a handler body.
