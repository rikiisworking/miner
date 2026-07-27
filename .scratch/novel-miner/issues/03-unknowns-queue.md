# 03 — Mark unknowns → durable queue

**What to build:** On the analyzed sentence screen, tapping a content-word row immediately saves that surface form as an unknown. The **first** successful unknown for a working sentence creates a **new queue entry with a new stable id** (never merge by sentence text). Analyze/browse alone does **not** write the queue. Further unknowns on that entry append in first-tap order; duplicate surface on the same entry is ignored. Multiple different unknowns on one entry are allowed. The learner can open a queue view of sentences and unknowns. **No** per-unknown or per-entry remove in v1 (Clear all lands in ticket 04). Queue survives server restart.

**Blocked by:** 02 — Sentence analyze (paste path): furigana + content-word list

**Status:** ready-for-agent

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [ ] Tap content-word row adds surface as unknown; first unknown creates new entry id + sets first-unknown-at
- [ ] Analyze without tapping creates no queue entry
- [ ] Identical sentence text mined again creates a **separate** entry (no merge-by-text)
- [ ] Duplicate tap of the same surface on the same entry does not create a second unknown
- [ ] Learner gets clear feedback on save and on ignored duplicate
- [ ] Queue view shows each entry (sentence + unknowns)
- [ ] No remove-unknown / remove-entry controls in v1
- [ ] Queue persists across process restart (QueueStore)

### Testing (required this ticket)

**L1 unit / facade**

- [ ] AddUnknown creates entry on first save; sets first-unknown-at
- [ ] Analyze-only path leaves store empty
- [ ] Second AddUnknown same surface same entry → no duplicate
- [ ] Two working sentences with same text → two entry ids
- [ ] Persistence contract: add → “restart” (new app instance, same store) → list still has entries (prefer real temp SQLite/file store for at least one test)

**L2 HTTP smoke**

- [ ] Authenticated add-unknown → queue list reflects entry
- [ ] Duplicate add → success or idempotent response; still one unknown

**L3 UI click smoke**

- [ ] Analyze fixture sentence → click content-word → save feedback visible
- [ ] Open queue → entry + unknown visible
- [ ] Click same word again → duplicate feedback; count unchanged

**Gate**

- [ ] New tests committed with feature
- [ ] Full suite (01–03) run green before ticket done
