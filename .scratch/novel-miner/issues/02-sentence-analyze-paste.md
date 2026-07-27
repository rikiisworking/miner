# 02 — Sentence analyze (paste path): furigana + content-word list

**What to build:** From the authenticated phone UI (**HTMX** partials over **Fiber**), the learner pastes or types a Japanese sentence (or short text), can edit it, and runs analysis. They see the sentence with furigana and a list of content words under it, each showing surface form and reading. Particles and similar function words are omitted from the list per the baseline content-word filter. Prefer HTMX request → HTML fragment swap for analyze results (not a JSON SPA). No camera or queue yet—this proves reading aid and the JapaneseAnalyzer port end-to-end.

**Blocked by:** 01 — LAN app shell + PIN gate

**Status:** ready-for-agent

**Parent:** `.scratch/novel-miner/spec.md`

**Content-word baseline (product rule):** Keep nouns, verbs, adjectives, adjectival nouns (and similar content classes). Drop particles, auxiliary verbs, symbols, punctuation, pure function words. Exact engine POS tags are documented on the analyzer adapter; tests use stub tokens with an explicit content vs non-content flag.

### Feature

- [ ] Authenticated user can enter/edit sentence text and trigger analysis
- [ ] Analyzed sentence is displayable with **HTML ruby** furigana (readings aligned to surfaces)
- [ ] Content-word list shows surface + reading only for content words (not bare particles)
- [ ] Baseline filter rule is applied/documented (stub flag in tests; adapter maps real tags later)
- [ ] MiningApp AnalyzeSentence (or equivalent) is the behavior under test; analyzer is a port
- [ ] Analysis failure surfaces a clear error in the UI

### Testing (required this ticket)

**L1 unit / facade**

- [ ] Analyze with fake analyzer returns tokens needed for furigana + content list
- [ ] Content-word filter: stub content tokens kept; particle/function stubs omitted
- [ ] Analyzer error surfaces as controlled failure from MiningApp (no uncaught throw into silence)

**L2 HTTP smoke**

- [ ] Authenticated analyze request with fixture sentence → 200 + expected payload shape
- [ ] Unauthenticated analyze → rejected
- [ ] Analyze failure path returns clear error status/body

**L3 UI click smoke**

- [ ] PIN unlock (reuse) → type/paste sentence → click analyze → HTML ruby (or equivalent reading aid markup) visible
- [ ] Content-word rows visible for stubbed content tokens; no bare particle-only rows for fixture
- [ ] Force analysis error (fake/test hook or bad path) → error message visible

**Gate**

- [ ] New tests committed with feature
- [ ] Full suite (01+02) run green before ticket done
