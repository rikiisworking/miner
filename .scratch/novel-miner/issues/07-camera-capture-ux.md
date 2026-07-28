# 07 — Phone camera capture UX

**What to build:** On top of file-upload OCR (ticket 06), add comfortable **in-browser camera capture** for the phone so the learner can shoot a novel page without leaving the app. Upload path remains available. Focus on capture controls, permissions/errors, and one-hand usability—not new product rules. OCR/glue behavior stays that of ticket 06.

**Blocked by:** 06 — Photo ingest + local OCR → same pipeline (file upload)

**Does not block:** 08 — Novel UX hardening (08 may ship on file-upload path alone)

**Status:** in progress

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [x] Authenticated UI supports camera capture on typical mobile browsers (in addition to file upload)
- [x] Captured image uses the same IngestPage / OCR path as file upload
- [x] Clear messaging when camera permission denied or capture unavailable; file upload still offered
- [x] Capture controls usable one-handed on a phone (large targets, minimal chrome)
- [x] No new MiningApp business rules

### Testing (required this ticket)

**L1 unit / facade**

- [x] No new business rules required; existing L1 suite still green (no regressions)
- [x] If any thin “capture result → IngestPage” helper exists, unit-test that it forwards bytes only *(none — client posts same multipart form)*

**L2 HTTP smoke**

- [x] No new routes required ideally; if capture posts same ingest endpoint, existing 06 smoke still green
- [x] Any new endpoint covered with one smoke *(no new endpoints)*

**L3 UI click smoke**

- [x] Authenticated shell shows camera capture control **and** file upload control
- [x] File upload path still completes (regression click: fixture image → candidates)
- [x] Camera control is present and clickable in headless env **or** gracefully shows fallback when `getUserMedia` unavailable (assert fallback/upload still usable—do not fail CI for missing hardware)
- [x] Do **not** require real webcam in CI

**Manual (required for camera hardware)**

- [ ] On learner’s phone: grant camera → capture → reaches sentence pick
- [ ] Deny permission → message + upload still works

**Gate**

- [ ] New/updated tests committed with feature
- [x] Full suite (01–07) run green before ticket done
