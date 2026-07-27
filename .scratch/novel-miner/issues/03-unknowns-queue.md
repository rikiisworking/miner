# 03 — Mark unknowns → durable queue

**What to build:** On the analyzed sentence screen, tapping a content-word row immediately saves that surface form as an unknown. The **first** successful unknown for a working sentence creates a **new queue entry with a new stable id** (never merge by sentence text). Analyze/browse alone does **not** write the queue. Further unknowns on that entry append in first-tap order; duplicate surface on the same entry is ignored. Multiple different unknowns on one entry are allowed. The learner can open a queue view of sentences and unknowns. **No** per-unknown or per-entry remove in v1 (Clear all lands in ticket 04). Queue survives server restart.

**Blocked by:** 02 — Sentence analyze (paste path): furigana + content-word list

**Status:** done

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [x] Tap content-word row adds surface as unknown; first unknown creates new entry id + sets first-unknown-at
- [x] Analyze without tapping creates no queue entry
- [x] Identical sentence text mined again creates a **separate** entry (no merge-by-text)
- [x] Duplicate tap of the same surface on the same entry does not create a second unknown
- [x] Learner gets clear feedback on save and on ignored duplicate
- [x] Queue view shows each entry (sentence + unknowns)
- [x] No remove-unknown / remove-entry controls in v1
- [x] Queue persists across process restart (QueueStore)

### Testing (required this ticket)

**L1 unit / facade**

- [x] AddUnknown creates entry on first save; sets first-unknown-at
- [x] Analyze-only path leaves store empty
- [x] Second AddUnknown same surface same entry → no duplicate
- [x] Two working sentences with same text → two entry ids
- [x] Persistence contract: add → “restart” (new app instance, same store) → list still has entries (prefer real temp SQLite/file store for at least one test)

**L2 HTTP smoke**

- [x] Authenticated add-unknown → queue list reflects entry
- [x] Duplicate add → success or idempotent response; still one unknown

**L3 UI click smoke**

- [x] Analyze fixture sentence → click content-word → save feedback visible
- [x] Open queue → entry + unknown visible
- [x] Click same word again → duplicate feedback; count unchanged

**Gate**

- [x] New tests committed with feature
- [x] Full suite (01–03) run green before ticket done
