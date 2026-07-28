# Integrated refactor plan — cleanup-2

**Branch:** `chore/cleanup-2` (from `origin/main` @ `5c95e60`, post tickets 01–08 + PR #8 cleanup-refactor)  
**Date:** 2026-07-28  
**Sources (parallel agents):**

| Agent | Focus |
|-------|--------|
| Architecture deepen (`/improve-codebase-architecture` vocab) | Seams, shallow modules, locality |
| Critical `/caveman-review` | Bugs + risks before large refactors |
| Test coverage | L1/L2/L3 gaps, concrete tests |
| Dead code + low fruit | Safe deletes, S/M refactors |

**Coverage baseline (no host tesseract):** total ~73.6% · `internal/app` 94.1% · `httpapi` 79.2% · `ocr` 35% (Recognize skipped) · `cmd/miner` 22.4%  
**Tools:** `go test ./...` green · `staticcheck` clean · `deadcode -test` clean · `ineffassign` 3 hits in handlers

---

## Goals

1. Fix **Pass-protocol races** around ClearAll + stale `entry_id` (only 🔴 bugs).
2. Close **L2 transport gaps** that mask product rules (auth shape, concurrent pass, ingest cancel, store errors).
3. Ship **safe mechanical cleanup** (ineffassign, unexport, string dedupe).
4. Deepen small **MiningApp locality** items (queue display sort, OCR deadline ownership).
5. Leave **feature work** (real JapaneseAnalyzer) as explicit later ticket — not this cleanup PR series.

**Non-goals (this plan):** multi-process queue, CSS redesign as product, TLS/HTTPS, real morph engine, preprocess/deskew, bulk OCR tag prune, Fiber `Immutable: true` global (pass already clones).

**Already done (do not re-schedule):** pass_id clone, ocr.Static seam, product NormalizePageText, ingest ctx/busy/cancel, QueueStore AppendResult + slim port, file splits, DC-1…6 from prior integrated plan.

---

## Priority stack (do in order)

| Phase | Theme | IDs | Why first |
|-------|--------|-----|-----------|
| **P0** | Critical correctness | CR-1, CR-2 | Pass protocol break under clear+mark; stale entry_id |
| **P1** | Tests that lock P0 + thin branches | TC-P0-* | Red before green; no refactor without net |
| **P2** | Security / ops risks (product-small) | CR-3…CR-6 | LAN PIN, session lifetime, queue perms, panic recover |
| **P3** | Mechanical dead / fruit | D1–D6, R1–R3, R10 | Zero behavior risk |
| **P4** | Architecture deepen (Worth exploring) | AR-1…AR-4 | Queue sort, timeouts, test double, camera JS |
| **P5** | Coverage depth | TC-P1/P2 | After seams stable |
| **P6** | Optional polish | R7–R12, AR-rest | Only if time |

---

## Phase P0 — Critical bugs (must before large refactor)

### CR-1 — Serialize ClearAll with pass bind/create

| | |
|--|--|
| **From** | caveman-review 🔴 |
| **Where** | `internal/app/export.go` `ClearAll`; `pass.go` `lookupOrCreate`; `unknowns.go` create path |
| **Bug** | `ClearAll` clears store then `passes.clear()` with **no coordination** vs concurrent `AddUnknown`. Window: create+bind then clear drops bind → same `pass_id` creates **second** entry (Pass protocol break). |
| **Fix** | Single critical section for “pass registry + clear bindings”. Preferred shape: hold pass mu, clear map **and** block `lookupOrCreate` for duration of durable `queue.ClearAll`, **or** clear map first under lock that `lookupOrCreate` also takes for bind+create. Document invariant: ClearAll and first-tap create never interleave. |
| **Test** | L1: goroutine A loops AddUnknown same pass; goroutine B ClearAll; after join, either empty queue **or** entries consistent with remaining pass binds (no orphan bind; no two entries for one logical pass after clear). At minimum: concurrent ClearAll + first-tap cannot leave pass map pointing at deleted IDs. |
| **Effort** | S–M |

**Caveman:**  
`export.go:ClearAll` + `pass.go:lookupOrCreate`: 🔴 bug: clear drops pass map without excluding concurrent create/bind. Serialize clear with pass registry.

### CR-2 — Stale `entry_id` after Clear all

| | |
|--|--|
| **From** | caveman-review 🔴 |
| **Where** | `unknowns.go` entry_id-wins path; Mine UI OOB `#entry_id`; clear is on Queue tab |
| **Bug** | Non-empty `entry_id` always wins over `pass_id`. After Clear all, Mine tab still has OOB entry id → append → `ErrEntryNotFound` (404); pass unused. Multi-tab clear+mark broken until re-analyze. |
| **Fix (pick one or both):** | **(A)** On `ErrEntryNotFound` with non-empty `pass_id`, fall back to pass create/bind (heal). **(B)** After ClearAll success, client drops `#entry_id` (redirect already reloads queue; Mine shell may keep OOB — clear hidden field via HTMX event or document “re-analyze after clear”). Prefer **A** in MiningApp for protocol locality + **B** if cheap. |
| **Test** | L1: Create via pass, ClearAll, AddUnknown with old entry_id + same pass_id → new entry (or clean error + no half-state). L2 optional: POST clear then mark with stale entry_id. |
| **Effort** | S |

**Caveman:**  
`unknowns.go:entry_id wins`: 🔴 bug: after ClearAll, stale entry_id 404s; pass ignored. Fallback to pass_id on not-found, and/or clear client entry_id.

---

## Phase P1 — Tests that lock P0 + known thin branches

Add **before or with** the fix (red → green).

| ID | Name / assert | Layer |
|----|---------------|-------|
| **TC-P0-1** | Concurrent ClearAll vs same-pass first-taps → no protocol break | L1 |
| **TC-P0-2** | Stale entry_id after ClearAll + pass_id heals or fails cleanly | L1 |
| **TC-P0-3** | `TestAddUnknown_SamePass_ConcurrentFirstTaps_HTTP` — 2× POST `/unknowns` same pass → 1 entry, 2 surfaces | L2 |
| **TC-P0-4** | `TestIngest_Canceled_RequestTimeout408` — cancel/deadline → 408 + candidates error; next ingest OK | L2 |
| **TC-P0-5** | `TestRequireAuth_HTML_RendersPinPage` — Accept text/html, no cookie → 401 + pin, not bare body | L2 |
| **TC-P0-6** | `TestRequireAuth_HTMX_PageText_AndIngest_AuthErrorOnly` — HX-Request, no session → `auth-error` fragment only | L2 |
| **TC-P0-7** | `TestQueuePage_NonEmpty_ClearConfirmAttribute` — N≥1 clear button `confirm(` + count wording | L2 |
| **TC-P0-8** | `TestIngestPage_DeadlineExceeded_IsCanceled` — short parent ctx → `ErrIngestCanceled`; busy free | L1 |

**Gate:** `go test ./internal/app ./internal/httpapi -count=1`

---

## Phase P2 — Risks (product-small, before more structure churn)

| ID | Issue | Fix | Effort |
|----|--------|-----|--------|
| **CR-3** | PIN unlock no rate limit; bind `:8080` LAN | Per-IP / simple backoff on `POST /unlock` (MiningApp or httpapi counter). Document in CONTEXT. | M |
| **CR-4** | Session “until restart” vs Fiber default **24h** Expiration | Set explicit `session.Config.Expiration` (0 / process-lifetime if Fiber allows) **or** update CONTEXT to 24h. Prefer match product intent. | S |
| **CR-5** | `queue.json` mode `0o644` world-readable | Write `0o600`; dir `0o700` if creating data dir | S |
| **CR-6** | No recover middleware; panic kills process | Fiber `recover` + HTMX-safe 500 fragment | S |
| **CR-7** | Ingest single-flight **after** full multipart buffer | Optional early busy probe before `ReadAll` (MiningApp `IngestBusy() bool` or atomic flag) | S–M |
| **CR-8** | Fiber BodyLimit 413 not mapped to candidates partial | Custom `ErrorHandler` for 413 + HX-Request | S |

**Defer (document only, not this cleanup unless product asks):** CSRF token for clear; client invent surface/sentence (trust shared PIN); unbounded pass map TTL; multi-process file; CookieSecure without TLS.

**Top-5 must-fix (caveman) covered by:** CR-1, CR-2, CR-3, CR-4, CR-6+CR-7.

---

## Phase P3 — Mechanical dead code + low fruit

| ID | Change | Risk |
|----|--------|------|
| **D1 / R1** | Fix ineffassign defaults in `handleAnalyze` / `handleAddUnknown` (switch, no dead defaults) | None |
| **D2** | Unexport `MultipartOverhead` → `multipartOverhead` | None |
| **D3** | Unexport `ocrtest.Manifest.ByID` → `byID` | None if only `Must` uses it |
| **D4** | Collapse duplicate HTML extract helpers in `server_test.go` | None |
| **D5** | Drop unused `class="shell"` or wire CSS | None |
| **D6 / R3** | One “image too large” user string (prefer product path → `renderIngestError`) | UX only |
| **R10** | Collapse duplicate default / `ErrOcrFailed` message in `renderIngestError` | None |

**Do not delete:** Mem/Static/MustEngine/ocrtest (test seams); Stub analyzer (prod until real engine); `PageIngest.Text` / `AddUnknownResult` fields used by L1.

**Gate:**

```bash
go test ./... -count=1 -timeout 120s
staticcheck ./...
deadcode -test ./...
ineffassign ./...
```

---

## Phase P4 — Architecture deepen (Worth exploring)

Use deep-module language: Module · Interface · Seam · Adapter · Depth · Locality · Leverage.

### AR-1 — Queue display order = export order (Strong product locality)

| | |
|--|--|
| **Problem** | `ExportMarkdown` sorts by `FirstUnknownAt` + id. `ListQueue` is shallow passthrough → Queue UI order = store accident. |
| **Solution** | Deepen `ListQueue` (or shared pure `sortQueueEntries`) with same order as export. QueueStore.List stays unordered. |
| **Benefits** | Locality of “what order means”; L1 test without HTTP. |
| **Effort** | S |
| **Test** | L1: create out-of-order times → ListQueue matches Export order. |

### AR-2 — One owner for ingest ceiling (OCR dual 60s)

| | |
|--|--|
| **Problem** | `MaxIngestDuration` and `defaultOCRTimeout` both 60s; independently editable → cancel/busy confusion. |
| **Solution** | Product owns ceiling via parent ctx. Adapter: shorter safety only, or honor parent only (no second 60s). Document on OcrEngine. |
| **Effort** | S |
| **Test** | Existing cancel tests + comment; no behavior change if nested min(deadline) preserved. |

### AR-3 — Unify analyzer test double

| | |
|--|--|
| **Problem** | L1 `fakeAnalyzer` vs `analyzer.Stub` (L2/e2e) — dual maintenance. |
| **Solution** | Extend Stub (map override + fail hooks) **or** export one controllable Adapter; delete package-local fake. |
| **Effort** | M |
| **Benefits** | Leverage across L1/L2/L3. |

### AR-4 — Camera script Module (static JS)

| | |
|--|--|
| **Problem** | ~180 lines camera IIFE in `shell.html` — no unit Seam; only L3. |
| **Solution** | `web/static/camera.js`; shell loads script; same multipart `/ingest`. Optional pure helpers (error name → message) if extracted without DOM. |
| **Effort** | M |
| **Test** | L3 still; optional tiny pure unit tests. |

### Parked (feature / speculative — not cleanup-2)

| ID | Item | Why park |
|----|------|----------|
| **AR-F1** | Real JapaneseAnalyzer Adapter + POS→Content | Product feature (P1 Depth gap, separate ticket) |
| **AR-F2** | Content-word rule ownership move to MiningApp | Pair with real analyzer |
| Unlock deepen, pure export peel, create-outside-lock, clock inject | Speculative / no pain |

---

## Phase P5 — Coverage depth (after P0–P3)

| ID | Test | Layer |
|----|------|-------|
| **TC-P1-1** | `SplitSentences` consecutive terminators → no empty candidates | L1 |
| **TC-P1-2** | `NormalizePageText` CRLF + `\u3000` + blank-line collapse | L1 |
| **TC-P1-3** | `AddUnknown` AppendUnknown store error propagates | L1 |
| **TC-P1-4** | File `save` failure (unwritable parent) | Adapter |
| **TC-P1-5** | ClearAll / Export store error → HTTP path exercises `renderUnexpected` | L2 |
| **TC-P1-6** | `resolveWebFS` embed + `MINER_WEB_ROOT` + missing path | cmd (export helper if needed) |
| **TC-P2-1** | `IngestPage(nil ctx)` OK | L1 |
| **TC-P2-2** | File concurrent append many surfaces | Adapter |
| **TC-P2-3** | Optional CI with tesseract + `MINER_OCR_CONTRACT=1` | CI |

**Coverage targets (aspirational, not vanity):**

- `internal/httpapi` ≥ ~85% (lift `renderUnexpected`, auth matrix, cancel 408)
- Keep `internal/app` ≥ 94%
- Do not chase `main` / `Listen` / real Tesseract % on hosts without binary

---

## Phase P6 — Optional polish

| ID | Item |
|----|------|
| **R6** | Rename `auth_error` define → `htmx_error` / `error_fragment` (non-auth 500s reuse it) |
| **R7** | Split mega test files (`mining_app_*`, `server_*`, `e2e/*`) by theme |
| **R8** | Shared `web/static/app.css` (pin/queue/shell) — design-light |
| **R11** | `make lint` → staticcheck + ineffassign + deadcode -test |
| **R12** | README: note `ocr.Static` test seam (CONTEXT already accurate) |
| **CR-doc** | Pass map growth / trust client surface — CONTEXT “known limits” |

---

## Suggested PR / commit slices (tiny, always green)

Follow Fowler: each step leaves suite green.

1. **test:** L1 concurrent ClearAll vs pass bind (red) → implement CR-1 → green  
2. **test:** L1 stale entry_id + pass heal (red) → CR-2 → green  
3. **test:** L2 concurrent same pass_id; auth HTML/HTMX matrix; ingest cancel 408; clear confirm HTML  
4. **fix:** session Expiration explicit + queue file `0o600` + recover middleware  
5. **fix:** (optional) unlock rate limit; early ingest busy; 413 ErrorHandler  
6. **refactor:** ineffassign handlers; unexport MultipartOverhead/ByID; single oversize string; renderIngestError collapse  
7. **refactor:** ListQueue sort = export; OCR timeout ownership comment/code  
8. **refactor:** unify analyzer test double  
9. **refactor:** camera.js extract  
10. **test:** P5 pure helpers + store error HTTP + resolveWebFS  
11. **chore:** make lint + README Static note + split tests if still noisy  

---

## Decision document

| Decision | Choice |
|----------|--------|
| Product rules stay on MiningApp | Unchanged (C1) |
| Pass protocol | Still never merge-by-text; one entry per pass_id until clear/restart |
| ClearAll concurrency | Must be serialized with pass bind/create |
| Stale entry_id | Prefer server heal via pass_id on not-found |
| Session lifetime | Explicit config matching CONTEXT (or CONTEXT update in same PR) |
| Queue file perms | Owner-only (`0o600`) |
| Real analyzer | Out of scope for cleanup-2 |
| Test doubles | Prefer one Analyzer Adapter for all layers |
| Queue order | Product sort in MiningApp.ListQueue |
| OCR deadline | Product MaxIngestDuration owns user-visible cancel |

---

## Testing decisions

- **Good test:** external behavior through MiningApp (L1) or HTTP status + `data-testid` (L2); not private map internals except Fiber lifetime clone test.  
- **Prior art:** `TestAddUnknown_SamePass_ConcurrentFirstTaps_OneEntry`, `TestIngestPage_CancelReleasesBusy`, `TestAnalyze_Unauthenticated_HTMX_AuthErrorFragment`.  
- **OCR-real:** keep skip when no tesseract; do not force into default gate.  
- **e2e:** keep ship-gate + clear confirm; do not rely on L3 for races (L1/L2).

---

## Out of scope

- Real morphological JapaneseAnalyzer production Adapter  
- Multi-process durable queue / DB  
- Full CSS design system  
- PIN complexity / OAuth / multi-user accounts  
- TLS termination in-process  
- OCR deskew/preprocess  
- Bulk delete unused OCR taxonomy tags without docs pass  
- Re-doing prior cleanup-refactor items (clone, Static OCR, NormalizePageText, AppendResult)

---

## Architecture health (baseline after tickets 01–08)

| Seam | Status |
|------|--------|
| MiningApp | Deep overall — protect pass + ingest; deepen ListQueue sort |
| Ports | Narrow; leave |
| httpapi | Thin map; fix transport edges only |
| OcrEngine | Good product normalize locality; timeout dual-const cleanup |
| JapaneseAnalyzer | **Shallow prod Adapter (Stub)** — feature ticket, not cleanup |
| Camera | Logic in template — extract for locality |

**No `docs/adr/`** — vocabulary in `CONTEXT.md`. Update CONTEXT when session lifetime, rate limit, or ListQueue sort lands.

---

## Success criteria

- [ ] No known Pass-protocol race under concurrent clear+mark  
- [ ] Stale entry_id after clear does not strand Mine UI  
- [ ] `make test` green without host tesseract  
- [ ] `ineffassign` / `staticcheck` / `deadcode -test` clean  
- [ ] L2: HTMX auth fragment on ≥2 routes; concurrent pass HTTP; ingest cancel 408; non-empty clear confirm HTML  
- [ ] Queue list order matches export (if AR-1 taken)  
- [ ] CONTEXT.md matches session + any new product rules  

---

## Further notes

- Prior architecture HTML reports under `/tmp/architecture-review-*.html` are **stale** on IngestPage/BodyLimit (already shipped). Use this doc as source of truth for cleanup-2.  
- Scout raw output lives in agent session; this plan is the integrated deliverable.  
- Recommended first implementation slice: **CR-1 + CR-2 + TC-P0-1/2** on this branch.
