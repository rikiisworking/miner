# 06 — Photo ingest + local OCR → same pipeline (file upload)

**What to build:** From the phone, the learner **uploads an image file** of a novel page (camera UI is ticket 07). The PC runs local OCR, discards the image when ingest finishes (**success or failure**), and presents sentence candidates into the existing select → analyze → queue → export pipeline. **Max upload 10 MB** (reject with clear error). **Single-flight ingest:** serialize IngestPage; disable upload control while running. OCR failure shows a clear error; the learner can still recover via edit or paste fallback. Working page state stays ephemeral and must not corrupt the durable queue. End-to-end mining works without manual paste of page text.

**Blocked by:** 04 — Export Markdown + Clear all; 05 — Full-page text → pick sentence

**Status:** blocked

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [ ] Authenticated UI supports image file upload via Fiber multipart + HTMX-friendly form (camera capture not required here)
- [ ] IngestPage (or equivalent) runs local OCR via OcrEngine port and returns text/sentence candidates
- [ ] Uploads **> 10 MB** rejected with clear error; OCR not run
- [ ] Image bytes/temp files discarded after ingest finishes (success **or** failure)
- [ ] Only one ingest at a time; UI disables upload while in flight
- [ ] OCR output flows into sentence pick → analyze → mark unknowns → export without a separate product path
- [ ] New page/upload does not wipe the durable queue
- [ ] OCR failure is visible and non-destructive to the existing queue
- [ ] Working OCR page state is ephemeral; abandon leaves queue alone
- [ ] Primary material assumption (novel prose) is documented; non-novel images are best-effort

### Testing (required this ticket)

**L1 unit / facade**

- [ ] IngestPage with fake OcrEngine returning fixed text → proposed sentences (via splitSentences)
- [ ] Payload over 10 MB rejected; store/queue unchanged
- [ ] Successful ingest does not leave image bytes in durable store
- [ ] Failed OCR still discards image; queue unchanged
- [ ] Concurrent second ingest rejected or queued behind first (single-flight contract)
- [ ] Ingest does not clear queue
- [ ] No real OCR engine required in L1

**L2 HTTP smoke**

- [ ] Authenticated multipart (or equivalent) upload of tiny fixture image bytes → candidates returned
- [ ] Unauthenticated upload rejected
- [ ] Oversize upload → clear error status/body
- [ ] OCR failure response clear; subsequent queue list still intact

**L3 UI click smoke**

- [ ] PIN → upload fixture image file via file input → progress or result → sentence candidates visible
- [ ] Upload control disabled (or non-reentrant) while ingest in progress
- [ ] Click candidate → analyze path (reuse 02/05)
- [ ] Required: mark unknown + export still works after OCR path (one end-to-end UI journey)
- [ ] OCR fail fixture/hook → error visible; queue empty-state or prior entries unchanged

**Manual (not a substitute for L1–L3)**

- [ ] Spike note: real novel page vs chosen OCR engine quality (document outcome)

**Gate**

- [ ] New tests committed with feature
- [ ] Full suite (01–06) run green before ticket done
