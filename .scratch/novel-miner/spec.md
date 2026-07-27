# Spec: Novel miner (page capture → sentence unknowns → export)

Status: ready-for-agent

Feature slug: `novel-miner`

---

## Problem Statement

When reading Japanese novels, the learner constantly meets words and kanji they do not know. They can use general tools (for example Papago) to understand a page in the moment, but turning those encounters into study material is painful: photograph or OCR a page, then manually copy words into a memo app, then later rebuild Anki notes by hand. Existing SRS (Anki) already handles long-term review; what is missing is a **low-friction, local, phone-reachable capture pipeline** that keeps the **sentence context** and the **exact unknowns marked**, then exports a clean file for manual Anki work—without becoming a second Anki or a full translator.

## Solution

A **home-PC web application** opened from a **phone browser** on the local network. The learner authenticates with a shared PIN, photographs a novel page, and the PC runs **local OCR**. They **tap one sentence** (and can edit the text if OCR is wrong), see that sentence with **furigana** (HTML ruby), and see a list of **content words** (surface form + reading) under it. **Tapping a list row** immediately saves that surface form as an unknown on a **queue entry** (first tap creates the entry). Multiple unknowns per entry are allowed. The queue persists on the PC. **Export** downloads a **Markdown** file (nested list; order by first-unknown save time) and **does not** modify the queue. **Clear all** is a separate control: after confirm, it wipes the entire queue. Photos are discarded after OCR (success or fail; max upload 10 MB). The app does **not** provide English meanings, dictionary senses, translation, or Anki integration.

## User Stories

1. As a learner, I want to open the app from my phone’s browser on home Wi‑Fi, so that I can mine vocabulary while reading on the couch without installing a native app.
2. As a learner, I want the heavy work to run on my home PC, so that phone storage and CPU stay light and local NLP/OCR tools can be used.
3. As a learner, I want a shared PIN or password before using the app, so that other devices on my LAN cannot freely use my mining server.
4. As a learner, I want wrong PIN attempts to be rejected clearly, so that I know I am not authenticated.
5. As a learner, I want to take a photo of a novel page with my phone, so that I do not have to type Japanese prose by hand.
6. As a learner, I want to upload an existing image of a page if the camera control is awkward, so that capture still works on finicky browsers.
7. As a learner, I want full-page OCR run locally on the PC, so that page images do not need a cloud OCR service.
8. As a learner, I want OCR results quickly after upload, so that mining stays in flow while reading.
9. As a learner, I want to see the recognized page text broken into candidate sentences, so that I can pick the line I actually care about.
10. As a learner, I want to tap one sentence to select it, so that processing does not treat the whole page as one blob.
11. As a learner, I want to edit the selected sentence text when OCR is wrong, so that vertical novel OCR mistakes do not block mining.
12. As a learner, I want re-analysis after I edit the sentence, so that furigana and the vocab list match the corrected text.
13. As a learner, I want the selected sentence displayed with furigana using HTML ruby, so that I can read it without leaving the app for a reading aid.
14. As a learner, I want a list of content words under the sentence, so that I can scan likely vocabulary without noise from every particle.
15. As a learner, I want each list row to show surface form and reading (for example surface with reading available for display), so that I can tell similar words apart without English glosses.
16. As a learner, I want particles and similar function words omitted from the default content-word list, so that the list stays short enough to use on a phone.
17. As a learner, I want to tap a content-word row to mark it unknown, so that I never copy-paste into a memo app.
18. As a learner, I want each tap to save immediately, so that marking several unknowns in one sentence is fast.
19. As a learner, I want the saved unknown to be the **surface form as shown**, so that export matches what I saw in the novel (no forced dictionary-form conversion).
20. As a learner, I want tapping the same surface form again on the same queue entry to do nothing harmful (no duplicate), so that mis-taps do not pollute the queue.
21. As a learner, I want to mark several different unknowns on the same sentence, so that dense novel lines can be mined in one pass.
22. As a learner, I want the same surface form on two different queue entries (including two entries with identical sentence text) to stay independent, so that context and mining passes are preserved for later Anki work.
23. As a learner, I want a queue view of what I have saved, so that I can review mining results before export.
24. As a learner, I want the queue to group or clearly show each sentence with its unknowns, so that I understand export shape before downloading.
25. As a learner, I want **no** per-unknown or per-entry remove in v1, so that the phone UI stays simple (mis-tap recovery is Clear all + re-mine, or live with the entry until clear).
26. As a learner, I want a **Clear all** control on the queue that wipes every entry after I confirm, so that I can empty the queue myself after a successful download.
27. As a learner, I want the queue to persist across browser refresh and server restart, so that a mining session is not lost if the phone sleeps.
28. As a learner, I want to export my queue to a Markdown file **without clearing the queue**, so that I can re-export or clear only when I choose.
29. As a learner, I want each export block to contain the original sentence and its unknowns as nested list items, so that context and targets travel together.
30. As a learner, I want unknowns listed as bullets under their sentence (not a single `;`-joined cell), so that the file is easy to scan and copy.
31. As a learner, I want export to include only entries that have at least one unknown, so that empty selections do not clutter the file.
32. As a learner, I want export ordered by **first-unknown save time** (earliest first), so that re-exports are easy to diff mentally.
33. As a learner, I want sentence text that contains Markdown-special or newline characters handled safely in the export list item, so that the file stays readable and structure does not break.
34. As a learner, I want export in UTF-8, so that Japanese text survives on every device I use for Anki prep.
35. As a learner, I want page photos discarded after OCR finishes (success or failure), so that disk is not filled with book images and retention stays minimal.
35a. As a learner, I want uploads larger than **10 MB** rejected with a clear error, so that the home PC is not overwhelmed by huge files.
36. As a learner, I want the app not to require English meanings, so that capture stays fast and I can look up meaning elsewhere (or in Anki later).
37. As a learner, I want the app not to push cards into Anki, so that my existing Anki workflow stays under my control.
38. As a learner, I want the app not to run spaced repetition, so that it does not compete with Anki.
39. As a learner, I want clear error messages when OCR fails, so that I can retake the photo or fall back to editing text.
40. As a learner, I want clear error messages when analysis fails, so that I know the sentence could not be tokenized.
41. As a learner, I want the UI to be usable with one hand on a phone, so that large tap targets work for sentence and vocab rows.
42. As a learner, I want to bookmark the PC address, so that I do not retype the LAN URL every session.
43. As a learner, I want the PC to show or log how to reach it on the LAN (host/port), so that first-time phone setup is obvious.
44. As a learner, I want sessions to work only when the PC is on and reachable, so that I understand the home-server model (no cloud host required).
45. As a learner primarily reading novels, I want OCR and layout assumptions aimed at prose book pages, so that the common case is optimized even if manga/signs are imperfect.
46. As a learner, I want best-effort behavior on non-novel material without hard failure, so that an occasional textbook or sign photo does not crash the app.
47. As a learner, I want no required book title or source metadata fields, so that capture stays minimal.
48. As a learner, I want no cloud calls for dictionary or translation in the core flow, so that understanding aids stay local (furigana/tokenization only).
49. As a developer-agent, I want a single application facade for all use-cases, so that behavior can be tested without the browser.
50. As a developer-agent, I want OCR, analysis, storage, and auth behind ports, so that engines can be swapped without rewriting product rules.
51. As a developer-agent, I want deterministic tests for export shape and dedupe rules, so that regressions in the mining contract are caught early.
63. As a developer-agent, I want unit/facade tests, HTTP smoke tests, and UI click smoke tests to grow with each ticket, so that each step is proven before the next starts.
64. As a developer-agent, I want a single documented command that runs the full automated suite after every ticket, so that “green” is unambiguous.
65. As a developer-agent, I want UI click tests to cover basic learner button flows (PIN, analyze, mark, queue, export, clear all, upload), so that the mobile shell is not only unit-tested.
52. As a learner, I want feedback when an unknown is saved (for example a short confirmation), so that I know the tap registered.
53. As a learner, I want feedback when a duplicate tap is ignored, so that I am not confused why the list count did not change.
54. As a learner, I want to start a new page capture without losing the existing queue, so that a reading session can span many pages.
55. As a learner, I want temporary in-progress OCR/analysis state separate from the durable queue, so that abandoning a bad page does not corrupt saved items.
56. As a learner, I want the content-word list derived from the same analysis as furigana, so that readings and list rows stay consistent.
57. As a learner, I want unknown order under each sentence to follow first-tap order, so that the file reflects how I mined the sentence.
58. As a learner, I want export to leave the queue unchanged, so that download problems never wipe mining work.
59. As a learner, I want Clear all to ask for confirmation (including count N when N≥1), so that a fat-finger does not destroy a long session.
60. As a learner, I want Clear all disabled or a no-op when the queue is empty, so that empty-state UI stays calm.
61. As a learner, I want the product to remain usable if I still open Papago sometimes for full translation, so that reading aid in this app does not pretend to replace every translator use.
62. As a learner, I want empty queue export to be safe (empty Markdown document; no crash), so that export never crashes.
66. As a learner, I want only one page ingest at a time (control disabled while OCR runs), so that double-taps do not race working state.

## Implementation Decisions

### Product shape

- Single-user **home server** model: PC process serves a mobile-first web UI on the LAN.
- **No native app** and no PWA requirement for v1; mobile browser only.
- **Shared PIN/password** gate for all mining endpoints and UI routes that touch data.
- **Session:** after correct PIN, server sets an HTTP session cookie valid until **process restart** (no TTL, no required logout UI in v1). Restart forces re-PIN. Cookie flags: **`HttpOnly`**, **`SameSite=Lax`**. Do not require `Secure` in v1 (HTTPS on LAN deferred).
- Primary content target: **novel pages**; other image types are best-effort.
- **No meanings, translation, dictionary senses, lemma export, or Anki APIs** in v1.

### Tech stack (frozen)

| Layer | Choice | Notes |
|-------|--------|--------|
| Language / runtime | **Go** | Single binary home server; standard module layout |
| HTTP framework | **[Fiber](https://gofiber.io/)** (`github.com/gofiber/fiber`) | Thin adapter over **MiningApp**; routes return HTML (and file download for export), not a JSON SPA |
| Frontend | **HTMX** | Server-rendered HTML partials; progressive enhancement via `hx-*` attributes. No React/Vue/Svelte SPA for v1 |
| HTML templates | Go `html/template` (or equivalent server templates) | Fiber serves full pages and HTMX fragments from the same app |
| Session | Fiber-compatible cookie session (in-memory OK for v1) | Aligns with cookie-until-restart rule |
| Durable queue | Local store (SQLite or equivalent) behind **QueueStore** | Concrete driver is an adapter pick |
| OCR / analyzer engines | Local adapters behind ports | Engine brands still implementation picks; must stay local |

**Architecture with this stack**

- **MiningApp** and ports live in plain Go packages (no Fiber types in domain/use-cases).
- Fiber handlers: parse request → call MiningApp → render template / HTMX fragment / file.
- L1 tests: `go test` against MiningApp with fakes (no Fiber).
- L2 tests: Fiber `app.Test` (or httptest) against mounted routes with fake ports.
- L3 tests: headless browser against running (or test) server; click real HTMX-driven controls.

**Not in stack (v1):** separate Node frontend build, SPA framework, cloud BaaS, mandatory WASM OCR in browser.

### Architectural seam (primary)

- Implement a single application facade, **MiningApp**, that owns product use-cases. The HTTP layer is a thin adapter over MiningApp.
- MiningApp is the **primary automated test seam**. Prefer testing through MiningApp with fake ports over testing framework or UI internals.

### Ports behind MiningApp

- **PinAuth** — verify the shared secret (constant-time comparison against configured hash or secret as implementation chooses; secret comes from environment or local config, not hard-coded in the repo).
- **OcrEngine** — `image bytes → plain text` (and optionally soft line breaks). Local engine only for v1.
- **JapaneseAnalyzer** — `sentence text → tokens` each with at least: surface form, reading (kana), and features sufficient to classify content vs non-content.
- **QueueStore** — durable persistence of queue entries (each with stable id); survives process restart.

### Domain concepts (vocabulary for this feature)

- **Page capture** — one uploaded/captured image processed by OCR; image bytes/temp files always discarded when ingest finishes (success or failure). Uploads over **10 MB** rejected before OCR.
- **OCR text** — full recognized text for a page; used to propose sentences.
- **Sentence** — learner-selected (and possibly edited) string that is the unit of context for unknowns and export.
- **Token** — analyzer output unit for a span of the sentence.
- **Content word** — token shown in the vocab list. Baseline filter (product rule; exact POS tags are adapter-specific and documented with the chosen analyzer):
  - **Keep:** nouns, verbs, adjectives, adjectival nouns (na-adjectives), and similar content classes the adapter maps into those buckets.
  - **Drop:** particles, auxiliary verbs, symbols, punctuation, and pure function words.
  - Tests use stub tokens with an explicit content vs non-content flag; real adapters map engine tags into that flag using the baseline above.
- **Unknown** — a surface form the learner tapped from the content-word list; stored exactly as surface; not required to be dictionary form. No special-character reject list in v1.
- **Queue entry** — durable record with a **stable unique id**, sentence text, ordered unique unknown surfaces, and **first-unknown-at** timestamp (set when the entry first receives an unknown). No queue row until first successful unknown. Creating a new mining pass always creates a **new entry id** even if sentence text matches an existing entry (no merge-by-text). Equal timestamps tie-break by entry id ascending.
- **Export document** — UTF-8 Markdown: one top-level list item per exportable entry (sentence text), nested bullets for unknowns in first-tap order. Entries ordered by **first-unknown-at** ascending. Export never mutates the queue.
- **Clear all** — after explicit user confirm, deletes every queue entry. Disabled or no-op when queue empty. No per-unknown or per-entry remove in v1.

Example export shape:

```markdown
- 病院に行った。
  - 病院
  - 行った
- 今日は雨だ。
  - 雨
```

Avoid overloaded terms from other projects: this product does **not** use Card, SM-2 Review, lemma identity, or Article/RSS Source.

### Core behaviors (business rules)

- Selecting a sentence may use OCR-proposed boundaries; after edit, the edited string is canonical.
- **Sentence segmentation** for page text is a pure function (or small helper) `splitSentences(text) → string[]`, not part of OcrEngine. Baseline: split on Japanese sentence punctuation (`。！？` and fullwidth variants) with safe fallback — if split yields nothing useful, expose full text as one editable candidate. Exact edge cases may be refined in tickets; edit remains the safety net.
- AnalyzeSentence returns data needed to render **HTML ruby** furigana and the content-word list.
- AddUnknown(working-sentence flow, surface):
  - Browse/analyze alone never writes the queue.
  - First successful unknown on a working sentence **creates** a new queue entry with a new id and sets first-unknown-at; later unknowns on that entry append by entry id.
  - If surface already present on that **entry**, no duplicate; otherwise append (first-tap order).
- Same surface on different entries are independent; identical sentence strings do not merge.
- Export includes only entries with ≥1 unknown.
- Export format: Markdown nested list as in the example above (not CSV).
- Export order: **first-unknown-at** ascending (tie-break entry id).
- Sentence text in list items must not break list structure (implementation may flatten internal newlines to spaces or escape consistently; tested).
- **ExportMarkdown never clears or mutates the queue.**
- **ClearAll** (after confirm in UI): delete all queue entries. No-op if already empty. No fine-grained remove APIs in v1.
- IngestPage: reject body/image **> 10 MB**; run at most **one ingest at a time** (serialize; UI disables capture/upload while in flight). Always discard image bytes/temp files when ingest finishes (success or failure); do not write long-lived image files in v1.
- Working OCR/page state is ephemeral (memory or temp); abandon or new capture must not corrupt the durable queue.
- No source/book metadata fields.

### API-shaped interactions (logical; transport may be HTTP)

MiningApp (and thus the HTTP API) exposes roughly:

- Unlock / session establish with PIN (session cookie until process restart; HttpOnly + SameSite=Lax)
- IngestPage(image) → page id + OCR text + proposed sentences (10 MB cap; single-flight)
- SetWorkingSentence(page id or independent, text) / analyze
- AddUnknown / list queue / ClearAll
- ExportMarkdown() → file bytes or string (queue unchanged)

Exact HTTP paths and JSON field names are left to implementation, but must not change the business rules above.

### UI interactions

- Mobile-first: photo capture or file picker → progress → sentence list/tap → sentence detail with **HTML ruby** furigana + tappable content-word rows → queue screen → **Export** download and separate **Clear all** (confirm when N≥1).
- Immediate save on vocab row tap; visible confirmation; duplicate feedback.
- Sentence text editor with explicit apply/re-analyze control.
- Upload/camera controls disabled while ingest runs.
- File-upload ingest can ship before polished camera capture (see issue split). Hardening (08) may follow file OCR (06) without waiting on camera (07).

### Persistence

- QueueStore is durable (SQLite or equivalent local store is acceptable).
- Working OCR page state may be ephemeral; queue must not be.
- No multi-user accounts; one shared PIN for the deployment.

### Phasing guidance (tickets 01–08 refine these phases)

1. Skeleton server + PIN + empty mobile UI reachable on LAN  
2. Text-driven path (inject or paste OCR text) through analyze → queue → Markdown export (proves rules without OCR)  
3. Local OCR ingest (file upload first) into the same pipeline; then phone camera UX  
4. Novel UX hardening (errors, empty states, vertical OCR tuning)

A hidden or dev-only paste path may remain as a fallback even after OCR ships.

### Explicit non-choices (deferred)

- **Stack is frozen:** Go + Fiber + HTMX (see Tech stack). Do not substitute Express/Rails/React/etc. without a spec change.
- OCR engine brand and Japanese analyzer brand remain **implementation picks**—except: **local OCR**, **local analysis**, adapters behind ports.
- Template library detail (`html/template` vs `templ`, etc.) may be chosen in ticket 01 as long as Fiber serves HTMX-friendly HTML.
- L3 browser tool (Playwright, chromedp, etc.) is an implementation pick; must drive real HTMX UI.
- HTTPS on LAN, VPN/off-site access, and PWA installability are deferred.
- PIN rate limiting / lockout deferred for v1 home LAN.
- Logout UI deferred (restart clears session).

## Testing Decisions

### Hard rule: tests land with each ticket

- Every ticket **adds** automated tests for its new behavior **in the same change** as the feature.
- Every ticket is **done only when** the full automated suite (all layers that exist so far) is **run and green**.
- Later tickets must not break earlier tests (regression gate).
- Ticket 01 establishes Go module layout, Fiber app wiring, HTMX static/partials, test layout (L1/L2/L3), and a single documented command (e.g. `make test` wrapping `go test ./...` plus L3) that runs **all** automated layers. Later tickets extend that same command—do not invent a second undiscoverable entrypoint without updating docs.
- Prefer fakes for OcrEngine, JapaneseAnalyzer, QueueStore, and PinAuth in automated tests. Real engines are manual/spike only unless an optional adapter contract test is added.

### Three automated layers (all required; grow per ticket)

| Layer | What | How | Speed target |
|-------|------|-----|--------------|
| **L1 Unit / facade** | Product rules via **MiningApp** (and pure helpers like `splitSentences`) | `go test`; fake ports; no Fiber imports in domain tests | Fast; run on every ticket |
| **L2 HTTP smoke** | Auth + critical routes over Fiber | Fiber `app.Test` / httptest; session cookie; HTML or redirect assertions; no browser | Fast/medium |
| **L3 UI click smoke** | Real HTMX button/control flows a learner would tap | Headless browser against local Fiber server with fakes | Medium; still required each ticket that adds UI |

**L1** is the source of truth for business rules. **L2** proves the HTTP adapter wires auth and payloads. **L3** proves the mobile UI can complete the happy path with clicks—not visual regression, not full matrix of devices.

### What good L1 tests look like

- Test **external behavior** of MiningApp (and export string shape), not private helpers, POS tag enums, or UI component trees.
- Deterministic fixtures: fixed analyzer token lists and fixed OCR strings; do not require real novel photos in unit/CI tests.
- Assert product rules: PIN rejection, content-word filtering outcomes given stubbed tokens, dedupe of unknowns per entry, multi-unknown order, no merge of identical sentence strings across entries, export Markdown shape and first-unknown-at order, export leaves queue unchanged, ClearAll empties store, >10 MB ingest rejected, image not retained after ingest success or fail, single-flight ingest, persistence via real or fake store contract.

### What good L2 smoke tests look like

- Boot app (or test server) with test config and fake ports where possible.
- Wrong PIN → 401/403 or equivalent; no mining body.
- Correct PIN → session cookie; authenticated route reachable.
- As features land: analyze, add unknown, list queue, export content-type and body shape, upload ingest with fixture bytes.
- Keep suite small: one smoke per major route family, not exhaustive HTTP matrix.

### What good L3 UI click tests look like

- Drive the **real UI** in a headless browser: type, click, upload fixture file where relevant.
- Assert visible outcomes (error text, furigana/list presence, queue row, download or cleared empty state)—not pixel screenshots.
- Use stable selectors (`data-testid` or role/label); avoid brittle CSS chains.
- Mobile viewport size preferred (e.g. phone width) when cheap.
- **Do not** require a physical phone or real camera in CI. Camera permission paths may be manual (ticket 07); L3 can click “upload file” and any on-screen control that does not need hardware.
- Grow one **happy-path journey** over tickets (PIN → … → export) rather than many disconnected click scripts. Add one **negative** click path when the ticket introduces a user-visible error (wrong PIN, bad OCR, analysis fail, `;` reject) if not already covered.

### Cumulative journey (L3 target by ticket)

| After ticket | Minimum automated UI journey |
|--------------|------------------------------|
| 01 | Open app → wrong PIN rejected → correct PIN → authenticated shell visible |
| 02 | … → paste/type sentence → analyze → furigana + content-word list visible |
| 03 | … → click content-word → queue shows entry; duplicate click feedback; remove works |
| 04 | … → export download (queue still has entries) → Clear all → confirm → queue empty |
| 05 | … → paste multi-sentence page → click candidate → analyze UI |
| 06 | … → upload fixture image → sentence candidates → (reuse analyze/queue path); oversize rejected |
| 07 | Camera control visible + file upload still works; permission-denied path manual if needed |
| 08 | Empty/error states; empty queue after Clear all; failed OCR/analysis messaging |

### Modules / seams under test

- **Primary (L1):** MiningApp use-cases (all user-visible rules).
- **Secondary (L2):** HTTP adapter smoke (PIN, session, export content-type/body, ingest).
- **Secondary (L3):** Critical learner clicks/taps through the shipped screens.
- **Adapter contract tests (optional, not blocking core):** one golden image or text file per OCR/analyzer adapter when those adapters are added—separate from rule tests so engine swaps do not rewrite the suite.

### Per-ticket definition of done (applies to 01–08)

A ticket is incomplete until **all** of the following hold:

1. Feature acceptance checkboxes for that ticket are met.
2. New **L1** tests for new MiningApp/helper rules are written.
3. New **L2** and/or **L3** coverage for any new UI or HTTP surface is written (see that ticket’s Testing section).
4. The **full** automated suite is executed locally (documented single command) and is **green**.
5. No intentional skip of prior layers without documenting why in the ticket (default: never skip).

### Prior art

- Greenfield repo: **no existing test suite**. Ticket 01 creates the harness; later tickets only add cases.
- Do not import scheduling/review test patterns from other Japanese-learning codebases; domain rules differ.

### Manual / spike checks (outside automated suite; still required where noted)

- Real vertical novel page photos against chosen OCR engine (ticket 06/08).
- Phone browser camera on the learner’s device OS (ticket 07).
- LAN reachability from phone to PC (ticket 01 manual note; L2/L3 use localhost).

## Out of Scope

- English translation, machine translation UI, or dictionary glosses/senses  
- Cloud OCR, cloud dictionary, or LLM explanation features  
- AnkiConnect, `.apkg` generation, CSV/Markdown note-type mapping into Anki  
- In-app SRS, drills, or review grading  
- Crop-first OCR workflow as the primary path  
- Retaining page images after OCR  
- Book title / source / page number metadata  
- Native iOS/Android apps; required PWA  
- Multi-user accounts and per-user queues  
- Off-LAN hosting, public internet deployment, or mandatory HTTPS  
- Perfect manga bubble OCR, handwriting, or street-sign optimization  
- Lemma/dictionary-form normalization as a product requirement  
- Drag-select arbitrary substring marking (v1 is list-tap of content words only)  
- “Show all tokens” toggle (content words only in v1)  
- Competing with Papago as a general translator  
- CSV export (replaced by Markdown nested list)  
- Auto-clear queue on export (export and Clear all are separate)  
- Per-unknown or per-entry remove (v1 is Clear all only)  
- `;` reject on unknown surfaces (dropped; Markdown bullets need no delimiter)  

## Further Notes

- **Relationship to Papago:** This app replaces the **memo mining** pain and provides **local reading aid** (furigana + tokenization). It does not aim to replace Papago’s full translation quality. Learners may still use a translator when a sentence is opaque even with furigana.
- **Anki handoff:** Export is intentionally dumb (sentence + unknown surfaces as Markdown lists). Card design stays a human Anki decision. **Export does not clear**; learner **Clear all** after download when ready.
- **OCR risk:** Full-page vertical novel OCR is the hardest operational risk. Editable sentence text is the product safety net; Phase 2 (text path) should land product rules before engine perfection.
- **Filter risk:** Content-word-only lists will occasionally hide a desired item; v1 recovery is edit sentence / live with the filter—not drag-select. No per-entry remove in v1.
- **Issue tracker:** Spec at `.scratch/novel-miner/spec.md`. Implementation tickets under `.scratch/novel-miner/issues/` (01–08).
- **Seams agreement:** Confirmed with stakeholder — primary test seam is MiningApp; publish target is local `.scratch/novel-miner/`.
- **Ticket map:** 01 PIN shell → 02 analyze paste → 03 queue (add only) → 04 Markdown export + Clear all → 05 page text pick (∥ 03 after 02) → 06 OCR file ingest+glue → 08 hardening (after 06) ; 07 camera UX after 06 (does not block 08).
- **Test map:** Each ticket has Feature + Testing (L1/L2/L3) + Done-only-when gate. Ticket 01 owns harness + full-suite command. No ticket done without green suite.
- **Grill decisions (locked):** separate Export vs Clear all + confirm; no fine-grained remove; first unknown creates entry; drop `;` reject; 10 MB + always delete image; HTML ruby; HttpOnly+SameSite=Lax; serialize ingest; empty export OK / Clear disabled when empty; 08 blocks on 06 not 07.
- **Stack (locked):** Go + Fiber backend; HTMX frontend (server-rendered HTML partials).
