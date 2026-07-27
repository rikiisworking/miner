# 05 — Full-page text → pick sentence

**What to build:** The learner pastes full-page (multi-sentence) Japanese text, sees candidate sentences, and taps one to select it. Selection feeds the existing analyze flow (furigana + content-word list). Editing the selected sentence regenerates analysis. Segmentation is a pure helper `splitSentences(text) → string[]` (not OcrEngine): baseline split on Japanese sentence punctuation (`。！？` and fullwidth variants); if split is useless, expose full text as one editable candidate. Still text-only—no OCR—but establishes the page → sentence step the photo path will reuse.

**Blocked by:** 02 — Sentence analyze (paste path): furigana + content-word list

**Status:** ready-for-agent

**Parent:** `.scratch/novel-miner/spec.md`

### Feature

- [ ] Learner can paste multi-sentence page text
- [ ] App proposes candidate sentences via `splitSentences` (documented baseline punctuation)
- [ ] Tapping a candidate sets it as the working sentence and shows analyze UI from ticket 02
- [ ] Editing the selected sentence and re-applying regenerates furigana and content-word list
- [ ] Segmentation edge cases fail safely (editable full text remains available)
- [ ] Working/page paste state is ephemeral and does not write the durable queue until unknowns are marked (via 03 flow)

### Testing (required this ticket)

**L1 unit / facade**

- [ ] `splitSentences` on multi-sentence fixture → expected candidates
- [ ] Edge: no terminator / empty / single blob → safe list (at least one editable string or documented empty)
- [ ] Select/set working sentence → analyze with fake analyzer (integration through MiningApp)
- [ ] Edit + re-analyze returns new tokens for edited text
- [ ] Propose/select alone does not create queue entries

**L2 HTTP smoke**

- [ ] Authenticated page-text / propose-sentences endpoint (or equivalent) returns candidates
- [ ] Select/set working sentence then analyze reachable over HTTP

**L3 UI click smoke**

- [ ] PIN → paste multi-sentence page → candidate list visible
- [ ] Click one candidate → analyze UI with furigana/list for that sentence
- [ ] Edit sentence → re-analyze control → updated list visible

**Gate**

- [ ] New tests committed with feature
- [ ] Full suite run green (includes 01–04 if already merged; at minimum 01–02+05 on this branch) before ticket done
