# 08 — Novel UX hardening

**What to build:** Polish the full file-upload→export loop for real novel use: clear empty states, stronger save/duplicate feedback, clearer OCR and analysis errors, server-side hint for LAN URL/IP on startup, and light tuning for vertical prose OCR/UX friction discovered in practice. No new product scope—ship-quality pass on the pipeline from tickets 01–06 (camera 07 does **not** block this ticket).

**Blocked by:** 06 — Photo ingest + local OCR → same pipeline (file upload)

**Status:** blocked

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [ ] Empty queue, failed OCR, and failed analysis each have clear, phone-readable messaging
- [ ] Save and duplicate-unknown feedback is consistent and obvious on mobile
- [ ] After Clear all, UI reflects empty queue
- [ ] Export still leaves queue intact (regression)
- [ ] Server logs or console output include how to reach the app on the LAN (host/port guidance)
- [ ] At least one real novel-page photo path is manually verified end-to-end (upload/capture → export; Clear all when done)
- [ ] No new features beyond polish/hardening of existing flows

### Testing (required this ticket)

**L1 unit / facade**

- [ ] Full regression: auth, analyze filter, entry identity, export shape/order, export does not clear, ClearAll, OCR fail leaves queue, 10 MB reject (existing tests—add only if gaps found during polish)
- [ ] No product rule changes without new L1 coverage

**L2 HTTP smoke**

- [ ] Smoke suite still green for PIN, analyze, queue, export, clear, ingest
- [ ] Any hardened error payloads still machine-assertable

**L3 UI click smoke**

- [ ] Empty queue state visible (navigate queue with no entries)
- [ ] Failed analysis path shows phone-readable error (hook/fake)
- [ ] Failed OCR path shows phone-readable error (hook/fake)
- [ ] Duplicate-unknown feedback still visible on second click
- [ ] Full happy path still green: PIN → upload fixture → pick → analyze → mark → export (queue remains) → Clear all → confirm → empty
- [ ] Prefer one stable end-to-end journey test as the ship gate

**Manual**

- [ ] Real novel page photo: ingest → export → Clear all → queue empty
- [ ] Confirm startup log shows LAN host/port hint

**Gate**

- [ ] Full automated suite (L1+L2+L3 through 06+08) run green
- [ ] Manual novel E2E noted (short checklist result in ticket or README)
- [ ] No new scope beyond polish
