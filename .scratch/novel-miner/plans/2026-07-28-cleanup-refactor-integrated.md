# Integrated code update plan — cleanup / refactor / coverage

**Branch:** `chore/cleanup-refactor` (from `origin/main` @ `83de7db`)  
**Sources:** parallel scouts (dead code, refactor, coverage, architecture heavy+urgent, critical `/review`, critical `/caveman-review`)  
**Filters applied:** architecture → **heavy + urgent only**; reviews → **critical only** (🔴 bug). Caveman had **0 🔴**; formal review had **1 🔴**. Non-critical 🟡 review risks deferred.

Scout artifacts (scratch): `/tmp/grok-1000/plan-*-32311f15.md`

---

## Goals

1. Fix real Pass-protocol correctness under Fiber.
2. Green suite **without** host Tesseract for non-OCR paths (OcrEngine seam real again).
3. Small dead-code deletes + high-value cleanup.
4. Coverage for transport/error edges after harness unblocks.
5. Only Strong architecture deepen items that block tickets 07/08.

**Non-goals:** real JapaneseAnalyzer, multi-process queue DB, CSS redesign, PIN rate-limit product change, pure nits.

---

## Priority stack (do in order)

| Phase | Theme | IDs | Why first |
|-------|--------|-----|-----------|
| **P0** | Critical bug | CR-1 | Pass map + Fiber unsafe strings → dup entries / map UB |
| **P1** | Test seam + suite green | AR-1 ≡ RF-3 ≡ TC-1 | Unblocks all L1/L2/L3 measurement and CI without tesseract |
| **P2** | Dead code (safe) | DC-1,2,3,6 | Zero behavior risk; shrink noise |
| **P3** | Product deepen (OCR path) | AR-2, AR-3 | Vertical candidates + busy/cancel before camera (07) |
| **P4** | QueueStore contract harden | RF-1, RF-7 | Multi-bool API + Mem/File drift |
| **P5** | Missing tests | TC-2…TC-6, TC-5, TC-11, TC-12 | After P1; error/race transport |
| **P6** | Locality polish | RF-2, RF-6, RF-4/15, RF-5, DC-4/5 | Pass registry, errors, file splits, UI trim |
| **P7** | Optional later | RF-8…14, DC-7…12, low TCs | Only if time |

---

## Phase P0 — Critical fix

### CR-1 — Clone `pass_id` before `openPasses` store

| | |
|--|--|
| **From** | `/review` Issue 1 (only critical) |
| **Where** | `internal/app/mining_app.go:218` (`openPasses[passID] = …`); fed by `httpapi` `FormValue("pass_id")` |
| **Bug** | Fiber default `Immutable: false` → `FormValue` is request-buffer view. Queue path already `strings.Clone` (`queuestore/entry.go`); pass map does not. Buffer reuse → miss pass→entry bind (duplicate queue entries) or map key UB. |
| **Fix** | `m.openPasses[strings.Clone(passID)] = res.EntryID` (clone on lookup key path too if needed after store). Optionally Fiber `Immutable: true` or clone all form values at HTTP edge. |
| **Test** | L2: force buffer reuse or unit-test MiningApp with mutable backing string after return; assert second tap same logical pass still one entry. Existing L1 concurrent tests use safe Go literals — not enough alone. |
| **Effort** | S |

**Caveman (critical dump):**  
`internal/app/mining_app.go:218: 🔴 bug: openPasses stores Fiber FormValue pass_id without clone; buffer reuse breaks pass→entry bind. Clone passID.`

**Dropped from reviews (not critical, do not schedule as P0):**

- Multi-process `queue.json.tmp` clobber (🟡; single-process product)
- PIN brute-force / no rate limit (🟡; LAN shared-PIN model)
- Session fixation, CookieSecure false, fsync, 0644 queue file, unbounded openPasses

---

## Phase P1 — OCR test seam (AR-1 / RF-3 / TC-1)

**One change, three labels.** Architecture Strong + highest coverage gate.

| | |
|--|--|
| **Problem** | Dropping `ocr.Stub` forced `MustEngine` into every L1/L2/L3 helper. Host without tesseract: app 25%, httpapi 0%, e2e all fail — even Unlock/PIN. OcrEngine seam = one-adapter for product tests. |
| **Solution** | Second **test** adapter (`ocr.StaticText` / func double). Default `newApp` / `newTestServer` / non-photo e2e → fake. `MustEngine` only OCR-real tests (Skip if missing). Update CONTEXT testing table. |
| **Not** | Reintroduce production stub in `cmd/miner`. |
| **Gate** | `go test ./internal/app ./internal/httpapi ./e2e -count=1` green **without** tesseract (photo/OCR cases skip or use fake). |
| **Effort** | S–M |

---

## Phase P2 — Dead code (mechanical)

| ID | Change | Risk |
|----|--------|------|
| DC-1 | Drop unused `AddUnknownResult.Sentence` | Low |
| DC-2 | Drop unused template keys `Added`/`Created` in `handleAddUnknown` | Low |
| DC-3 | Drop dead HTMX branch in `handleClearQueue` (UI is plain POST) | Low; if keeping future HTMX clear → test instead of delete |
| DC-6 | Drop ineffectual `image = nil` | None |

**Gate:** `go test ./internal/app ./internal/httpapi`  
**Defer:** DC-4/5 (HTMX on pin/queue, dead CSS) → P6; DC-7…12 hygiene.

---

## Phase P3 — Architecture heavy + urgent (AR-2, AR-3)

### AR-2 — Product normalize in MiningApp (not Tesseract)

| | |
|--|--|
| **Problem** | `normalizeOCRText` / inter-CJK strip live in Tesseract → candidate quality adapter-secret; `IngestPage` shallow on page-text hygiene. |
| **Solution** | Pure helper next to `SplitSentences`; `IngestPage` always normalizes engine text before split. Tesseract returns engine stdout (trim only). Port docs: engine text ≠ product-normalized. |
| **Benefits** | Locality for ticket 08 vertical polish; fakes and future engines share rules. |
| **Effort** | M |

### AR-3 — Ingest ctx + busy release

| | |
|--|--|
| **Problem** | Single-flight product rule holds busy for full CLI; adapter uses `context.Background()` timeout only. Abandon/retry → stuck `ErrIngestBusy` up to 60s. Ticket 07 camera multiplies. |
| **Solution** | `IngestPage(ctx, …)` or product deadline owned by MiningApp; `OcrEngine.Recognize` honors ctx; cancel releases single-flight; clear product error. |
| **Tests** | L1 fake blocked on channel + cancel → busy clears (needs P1 fake). |
| **Effort** | M |

**Dropped architecture (not heavy+urgent):** real analyzer feature, ErrEmptyPage split alone, httpapi file size, QueueStore RMW, preprocess/deskew, pass-map growth, ocrtest MaxUploadBytes alias.

---

## Phase P4 — QueueStore API harden

| ID | Change |
|----|--------|
| RF-1 | `AppendUnknown` → `AppendResult{Entry, Added, Found}`; shared `ErrEmptyID` / `ErrDuplicateID`; Mem empty-id same as File |
| RF-7 | Shared contract test both backends |

**Effort:** M + S  
**Gate:** `go test ./internal/adapters/queuestore ./internal/app`

---

## Phase P5 — Tests to add (after P1)

| ID | Layer | What |
|----|-------|------|
| TC-2 | L2 | Ingest busy → 409 |
| TC-3 | L2 | Missing `image` field → 400 |
| TC-4 | L2 | Empty OCR text → 400 message |
| TC-5 | L1 | ClearAll drops `openPasses` bindings |
| TC-6 | L2 | AddUnknown empty surface / missing entry |
| TC-11 | L1 | Store errors propagate |
| TC-12 | adapter | Corrupt `queue.json` |

**Also:** regression for CR-1 (Fiber buffer / clone).  
**Then if time:** TC-8,10,14; skip low TC-16…23 unless touching that code.  
**Note:** TC-7 HTMX clear only if DC-3 **keeps** HTMX branch; if DC-3 deletes branch, drop TC-7.

**Coverage baseline (this host, no tesseract):** total **36.1%**; after P1 expect app/httpapi high from existing tests; then close error branches.

---

## Phase P6 — Refactor polish (ordered)

1. **RF-2** — Extract `passRegistry`; rename `generateID` → `newID` (do after CR-1 clone lives in registry).
2. **RF-6** — Split `ErrEmptyImage` vs `ErrEmptyPage` (small; pairs with TC-3/4).
3. **RF-4 + RF-15** — Split `httpapi` files same package; HTMX-friendly 500 fragment for store I/O.
4. **RF-5** — Split `app` files by concern (`ingest.go`, `unknowns.go`, …).
5. **DC-4 + DC-5** — Drop HTMX script on pin/queue; dead export CSS.
6. **RF-8, RF-9, RF-10…** — Tesseract defaults, upload helper, analyzer test double unify — opportunistic.

---

## Suggested PR / commit slices

Small green commits on `chore/cleanup-refactor` (or stacked PRs):

| Commit | Scope |
|--------|--------|
| 1 | **fix:** clone pass_id for openPasses (CR-1 + test) |
| 2 | **test:** OCR fake default harness (P1) + CONTEXT note |
| 3 | **chore:** dead code DC-1/2/3/6 |
| 4 | **refactor:** product OCR normalize into app (AR-2) |
| 5 | **feat/refactor:** ingest context + busy cancel (AR-3) |
| 6 | **refactor:** QueueStore AppendResult + contract (RF-1/7) |
| 7 | **test:** TC-2…6,11,12 |
| 8+ | RF-2, RF-6, httpapi/app file splits, DC-4/5 |

---

## Acceptance bar

- [ ] CR-1 fixed + regression exists
- [ ] Non-OCR L1/L2/L3 green without tesseract; OCR tests Skip or opt-in
- [ ] `staticcheck` / `deadcode -test` clean after deletes
- [ ] CONTEXT.md matches real test seams (fake OCR default; real engine for OCR-only)
- [ ] Handlers still pure transport (C1); no new public packages unless justified
- [ ] `make test` green on machine **with** tesseract for full OCR smoke

---

## Explicit exclusions (scouts found, user filters drop)

| Item | Why out |
|------|---------|
| Worth exploring / Speculative architecture | Filter: heavy+urgent Strong only |
| Review 🟡 multi-process tmp, PIN lockout, session, fsync, 0644 | Filter: critical only |
| DC-10 tag taxonomy, DC-12 CSS mega-dedupe | Low value / product UI |
| Production morphological analyzer | Feature ticket, not cleanup |
| Multi-process queue locking | Product stays single process |

---

## Scout index

| Scout | Path |
|-------|------|
| Dead code | `/tmp/grok-1000/plan-dead-code-32311f15.md` |
| Refactor | `/tmp/grok-1000/plan-refactor-cleanup-32311f15.md` |
| Coverage | `/tmp/grok-1000/plan-test-coverage-32311f15.md` |
| Architecture | `/tmp/grok-1000/plan-architecture-32311f15.md` |
| Review critical | `/tmp/grok-1000/plan-review-critical-32311f15.md` |
| Caveman critical | `/tmp/grok-1000/plan-caveman-review-32311f15.md` |
| Cover data | `/tmp/grok-1000/cover-*.txt`, `test-run-32311f15.txt` |
