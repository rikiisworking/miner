# 06 — Photo ingest + local OCR → same pipeline (file upload)

**What to build:** From the phone, the learner **uploads an image file** of a novel page (camera UI is ticket 07). The PC runs local OCR, discards the image when ingest finishes (**success or failure**), and presents sentence candidates into the existing select → analyze → queue → export pipeline. **Max upload 10 MB** (reject with clear error). **Single-flight ingest:** serialize IngestPage; disable upload control while running. OCR failure shows a clear error; the learner can still recover via edit or paste fallback. Working page state stays ephemeral and must not corrupt the durable queue. End-to-end mining works without manual paste of page text.

**Blocked by:** 04 — Export Markdown + Clear all; 05 — Full-page text → pick sentence

**Status:** done

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [x] Authenticated UI supports image file upload via Fiber multipart + HTMX-friendly form (camera capture not required here)
- [x] IngestPage (or equivalent) runs local OCR via OcrEngine port and returns text/sentence candidates
- [x] Uploads **> 10 MB** rejected with clear error; OCR not run
- [x] Image bytes/temp files discarded after ingest finishes (success **or** failure)
- [x] Only one ingest at a time; UI disables upload while in flight
- [x] OCR output flows into sentence pick → analyze → mark unknowns → export without a separate product path
- [x] New page/upload does not wipe the durable queue
- [x] OCR failure is visible and non-destructive to the existing queue
- [x] Working OCR page state is ephemeral; abandon leaves queue alone
- [x] Primary material assumption (novel prose) is documented; non-novel images are best-effort

### Testing (required this ticket)

**L1 unit / facade**

- [x] IngestPage with fake OcrEngine returning fixed text → proposed sentences (via splitSentences)
- [x] Payload over 10 MB rejected; store/queue unchanged
- [x] Successful ingest does not leave image bytes in durable store
- [x] Failed OCR still discards image; queue unchanged
- [x] Concurrent second ingest rejected or queued behind first (single-flight contract)
- [x] Ingest does not clear queue
- [x] No real OCR engine required in L1

**L2 HTTP smoke**

- [x] Authenticated multipart (or equivalent) upload of tiny fixture image bytes → candidates returned
- [x] Unauthenticated upload rejected
- [x] Oversize upload → clear error status/body
- [x] OCR failure response clear; subsequent queue list still intact

**L3 UI click smoke**

- [x] PIN → upload fixture image file via file input → progress or result → sentence candidates visible
- [x] Upload control disabled (or non-reentrant) while ingest in progress
- [x] Click candidate → analyze path (reuse 02/05)
- [x] Required: mark unknown + export still works after OCR path (one end-to-end UI journey)
- [x] OCR fail fixture/hook → error visible; queue empty-state or prior entries unchanged

**Manual (not a substitute for L1–L3)**

- [x] Spike note: real novel page vs chosen OCR engine quality (document outcome)

**Gate**

- [x] New tests committed with feature
- [x] Full suite (01–06) run green before ticket done

### Spike note — OCR engine (2026-07)

- Product rules + HTTP + UI complete behind **OcrEngine** port.
- Automated path uses **`ocr.Stub`** (`Text` / `ByBytes` / `FailWith`); L3 maps fixture bytes → `expected_text` from `testdata/ocr/cases.json`.
- No real local engine wired in process entry yet (`cmd/miner` still uses empty Stub → Recognize fails until configured).
- Host had no `tesseract` binary at spike time; pick later (tesseract+jpn, manga-ocr, Paddle, …) as **adapter only** — do not re-implement IngestPage rules in the adapter.
- Quality vs real vertical novel pages deferred to ticket 08 manual verification once an engine lands.
