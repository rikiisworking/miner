# 04 — Export Markdown + Clear all

**What to build:** From the queue, two separate controls:

1. **Export** — download UTF-8 Markdown. Each exportable entry (≥1 unknown) is a top-level list item (sentence text); unknowns are nested bullets in first-tap order. Entries ordered by **first-unknown-at** ascending (tie-break entry id). **Export never mutates the queue.**
2. **Clear all** — when queue non-empty, ask confirm (“Clear all N entries?”); on confirm wipe entire queue. When empty: control disabled or no-op (no confirm).

Example export:

```markdown
- 病院に行った。
  - 病院
  - 行った
- 今日は雨だ。
  - 雨
```

Empty queue export is safe (empty document; no crash). This completes the text-only mining path (memo replacement without camera).

**Blocked by:** 03 — Mark unknowns → durable queue

**Status:** done

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [x] Export produces UTF-8 Markdown nested list (sentence top-level item; unknowns nested bullets)
- [x] Only entries with ≥1 unknown appear
- [x] Entry order is first-unknown-at ascending (tie-break entry id)
- [x] Unknown order under a sentence is first-tap order
- [x] Sentence text with newlines/special characters does not break list structure (tested)
- [x] Same sentence text on two entries yields two blocks (entry identity preserved)
- [x] Empty queue export does not error (empty document OK)
- [x] Export leaves queue unchanged (re-export possible)
- [x] Clear all requires confirm when N≥1; on confirm queue empty
- [x] Clear all disabled or no-op when N=0
- [x] Phone UI exposes separate Export and Clear all controls (Fiber routes + HTMX-friendly markup; export may be file download / `Content-Disposition`)

### Testing (required this ticket)

**L1 unit / facade**

- [x] Export shape matches nested list contract (fixture entries)
- [x] Order by first-unknown-at ascending; tie-break entry id
- [x] Unknowns first-tap order under each sentence
- [x] Newline/special sentence text does not break structure
- [x] Empty queue → empty document, no throw
- [x] Export leaves store unchanged
- [x] ClearAll empties store; ClearAll on empty is no-op

**L2 HTTP smoke**

- [x] Authenticated export → body is UTF-8 Markdown; content-type sensible (`text/markdown` or documented equivalent)
- [x] After export, queue list still has same entries
- [x] Empty-queue export still 200 (or documented success) with empty body
- [x] ClearAll → queue empty; second ClearAll safe

**L3 UI click smoke**

- [x] Full text happy path: PIN → analyze → mark unknown(s) → open queue → click Export
- [x] After export, queue entries still visible
- [x] Click Clear all → confirm → queue empty state
- [x] Clear all control not usable / no-op when empty
- [x] Export control is a real clickable control; download or content receipt asserted as the stack allows

**Gate**

- [x] New tests committed with feature
- [x] Full suite (01–04) run green before ticket done — **text-only pipeline complete**
