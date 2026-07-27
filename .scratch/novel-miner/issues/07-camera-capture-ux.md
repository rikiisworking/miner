# 07 — Phone camera capture UX

**What to build:** On top of file-upload OCR (ticket 06), add comfortable **in-browser camera capture** for the phone so the learner can shoot a novel page without leaving the app. Upload path remains available. Focus on capture controls, permissions/errors, and one-hand usability—not new product rules. OCR/glue behavior stays that of ticket 06.

**Blocked by:** 06 — Photo ingest + local OCR → same pipeline (file upload)

**Does not block:** 08 — Novel UX hardening (08 may ship on file-upload path alone)

**Status:** blocked

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [ ] Authenticated UI supports camera capture on typical mobile browsers (in addition to file upload)
- [ ] Captured image uses the same IngestPage / OCR path as file upload
- [ ] Clear messaging when camera permission denied or capture unavailable; file upload still offered
- [ ] Capture controls usable one-handed on a phone (large targets, minimal chrome)
- [ ] No new MiningApp business rules

### Testing (required this ticket)

**L1 unit / facade**

- [ ] No new business rules required; existing L1 suite still green (no regressions)
- [ ] If any thin “capture result → IngestPage” helper exists, unit-test that it forwards bytes only

**L2 HTTP smoke**

- [ ] No new routes required ideally; if capture posts same ingest endpoint, existing 06 smoke still green
- [ ] Any new endpoint covered with one smoke

**L3 UI click smoke**

- [ ] Authenticated shell shows camera capture control **and** file upload control
- [ ] File upload path still completes (regression click: fixture image → candidates)
- [ ] Camera control is present and clickable in headless env **or** gracefully shows fallback when `getUserMedia` unavailable (assert fallback/upload still usable—do not fail CI for missing hardware)
- [ ] Do **not** require real webcam in CI

**Manual (required for camera hardware)**

- [ ] On learner’s phone: grant camera → capture → reaches sentence pick
- [ ] Deny permission → message + upload still works

**Gate**

- [ ] New/updated tests committed with feature
- [ ] Full suite (01–07) run green before ticket done
