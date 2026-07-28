# 05 — Full-page text → pick sentence

**What to build:** The learner pastes full-page (multi-sentence) Japanese text, sees candidate sentences, and taps one to select it. Selection feeds the existing analyze flow (furigana + content-word list). Editing the selected sentence regenerates analysis. Segmentation is a pure helper `splitSentences(text) → string[]` (not OcrEngine): baseline split on Japanese sentence punctuation (`。！？` and fullwidth variants); if split is useless, expose full text as one editable candidate. Still text-only—no OCR—but establishes the page → sentence step the photo path will reuse.

**Blocked by:** 02 — Sentence analyze (paste path): furigana + content-word list

**Status:** done

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [x] Learner can paste multi-sentence page text
- [x] App proposes candidate sentences via `splitSentences` (documented baseline punctuation)
- [x] Tapping a candidate sets it as the working sentence and shows analyze UI from ticket 02
- [x] Editing the selected sentence and re-applying regenerates furigana and content-word list
- [x] Segmentation edge cases fail safely (editable full text remains available)
- [x] Working/page paste state is ephemeral and does not write the durable queue until unknowns are marked (via 03 flow)

### Testing (required this ticket)

**L1 unit / facade**

- [x] `splitSentences` on multi-sentence fixture → expected candidates
- [x] Edge: no terminator / empty / single blob → safe list (at least one editable string or documented empty)
- [x] Select/set working sentence → analyze with fake analyzer (integration through MiningApp)
- [x] Edit + re-analyze returns new tokens for edited text
- [x] Propose/select alone does not create queue entries

**L2 HTTP smoke**

- [x] Authenticated page-text / propose-sentences endpoint (or equivalent) returns candidates
- [x] Select/set working sentence then analyze reachable over HTTP

**L3 UI click smoke**

- [x] PIN → paste multi-sentence page → candidate list visible
- [x] Click one candidate → analyze UI with furigana/list for that sentence
- [x] Edit sentence → re-analyze control → updated list visible

**Gate**

- [x] New tests committed with feature
- [x] Full suite run green (includes 01–04 if already merged; at minimum 01–02+05 on this branch) before ticket done
