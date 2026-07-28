# 08 — Novel UX hardening

**What to build:** Polish the full file-upload→export loop for real novel use: clear empty states, stronger save/duplicate feedback, clearer OCR and analysis errors, server-side hint for LAN URL/IP on startup, and light tuning for vertical prose OCR/UX friction discovered in practice. No new product scope—ship-quality pass on the pipeline from tickets 01–06 (camera 07 does **not** block this ticket).

**Blocked by:** 06 — Photo ingest + local OCR → same pipeline (file upload)

**Status:** done (automated); manual novel photo + LAN log checklist below

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [x] Empty queue, failed OCR, and failed analysis each have clear, phone-readable messaging
- [x] Save and duplicate-unknown feedback is consistent and obvious on mobile
- [x] After Clear all, UI reflects empty queue
- [x] Export still leaves queue intact (regression)
- [x] Server logs or console output include how to reach the app on the LAN (host/port guidance)
- [ ] At least one real novel-page photo path is manually verified end-to-end (upload/capture → export; Clear all when done)
- [x] No new features beyond polish/hardening of existing flows

### Testing (required this ticket)

**L1 unit / facade**

- [x] Full regression: auth, analyze filter, entry identity, export shape/order, export does not clear, ClearAll, OCR fail leaves queue, 10 MB reject (existing tests—add only if gaps found during polish)
- [x] No product rule changes without new L1 coverage
- [x] `formatLANHints` unit coverage (wildcard / loopback / explicit host / empty IPv4 list)

**L2 HTTP smoke**

- [x] Smoke suite still green for PIN, analyze, queue, export, clear, ingest
- [x] Any hardened error payloads still machine-assertable

**L3 UI click smoke**

- [x] Empty queue state visible (navigate queue with no entries)
- [x] Failed analysis path shows phone-readable error (hook/fake)
- [x] Failed OCR path shows phone-readable error (hook/fake)
- [x] Duplicate-unknown feedback still visible on second click
- [x] Full happy path still green: PIN → upload fixture → pick → analyze → mark → export (queue remains) → Clear all → confirm → empty
- [x] Prefer one stable end-to-end journey test as the ship gate (`TestUI_ShipGate_PhotoUpload_Export_ClearAll`)

**Manual**

- [ ] Real novel page photo: ingest → export → Clear all → queue empty
- [ ] Confirm startup log shows LAN host/port hint

**Gate**

- [x] Full automated suite (L1+L2+L3 through 06+08) run green (`make test` 2026-07-28)
- [ ] Manual novel E2E noted (short checklist result in ticket or README)
- [x] No new scope beyond polish

### Manual checklist (fill when run)

| Step | Result | Notes |
|------|--------|-------|
| `make run` with `MINER_PIN` — stderr shows Dev + LAN try URLs | | |
| Phone (same Wi‑Fi): unlock → upload real novel page → candidates | | |
| Pick → edit if OCR wrong → analyze → mark unknowns → Queue | | |
| Export Markdown → file has sentences/unknowns; Queue still full | | |
| Clear all → confirm → empty + Clear disabled | | |
