package app_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rikiisworking/miner/internal/adapters/ocr"
	"github.com/rikiisworking/miner/internal/adapters/pinauth"
	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/ports"
)

// fakeAnalyzer is a test double for ports.JapaneseAnalyzer.
// Controllable tokens stay package-local; queue/pin use shared adapters.
type fakeAnalyzer struct {
	// byText maps exact sentence text to tokens. Explicit Content flags on tokens.
	byText map[string][]ports.Token
	// failWith, when set, makes every Analyze call fail.
	failWith error
	// failOn makes Analyze fail only for that exact text.
	failOn string
}

func (f fakeAnalyzer) Analyze(text string) ([]ports.Token, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	if f.failOn != "" && text == f.failOn {
		return nil, errors.New("forced analyzer failure")
	}
	if f.byText != nil {
		if toks, ok := f.byText[text]; ok {
			return toks, nil
		}
	}
	return []ports.Token{{Surface: text, Reading: "", Content: true}}, nil
}

func newApp(t *testing.T, analyzer ports.JapaneseAnalyzer) *app.MiningApp {
	t.Helper()
	return newAppWithQueue(t, analyzer, queuestore.NewMem())
}

func newAppWithQueue(t *testing.T, analyzer ports.JapaneseAnalyzer, queue ports.QueueStore) *app.MiningApp {
	t.Helper()
	if analyzer == nil {
		analyzer = fakeAnalyzer{}
	}
	return app.NewMiningApp(pinauth.Static{Secret: "test-pin-ok"}, analyzer, queue, ocr.Stub{})
}

func newAppWithOCR(t *testing.T, o ports.OcrEngine) *app.MiningApp {
	t.Helper()
	return app.NewMiningApp(pinauth.Static{Secret: "test-pin-ok"}, fakeAnalyzer{}, queuestore.NewMem(), o)
}

func TestUnlock_AcceptsCorrectPIN(t *testing.T) {
	m := newApp(t, nil)

	err := m.Unlock("test-pin-ok")
	if err != nil {
		t.Fatalf("Unlock correct PIN: %v", err)
	}
}

func TestUnlock_RejectsWrongPIN(t *testing.T) {
	m := newApp(t, nil)

	err := m.Unlock("wrong")
	if !errors.Is(err, app.ErrInvalidPIN) {
		t.Fatalf("Unlock wrong PIN: got %v, want ErrInvalidPIN", err)
	}
}

func TestUnlock_RejectsEmptyWhenSecretSet(t *testing.T) {
	m := newApp(t, nil)

	err := m.Unlock("")
	if !errors.Is(err, app.ErrInvalidPIN) {
		t.Fatalf("Unlock empty PIN: got %v, want ErrInvalidPIN", err)
	}
}

func TestAnalyzeSentence_ReturnsTokensForFuriganaAndContentList(t *testing.T) {
	sentence := "私は本を読む。"
	tokens := []ports.Token{
		{Surface: "私", Reading: "わたし", Content: true},
		{Surface: "は", Reading: "", Content: false},
		{Surface: "本", Reading: "ほん", Content: true},
		{Surface: "を", Reading: "", Content: false},
		{Surface: "読む", Reading: "よむ", Content: true},
		{Surface: "。", Reading: "", Content: false},
	}
	m := newApp(t, fakeAnalyzer{byText: map[string][]ports.Token{sentence: tokens}})

	got, err := m.AnalyzeSentence(sentence)
	if err != nil {
		t.Fatalf("AnalyzeSentence: %v", err)
	}
	if got.Sentence != sentence {
		t.Fatalf("Sentence=%q want %q", got.Sentence, sentence)
	}
	if len(got.Tokens) != len(tokens) {
		t.Fatalf("Tokens len=%d want %d", len(got.Tokens), len(tokens))
	}
	for i := range tokens {
		if got.Tokens[i] != tokens[i] {
			t.Fatalf("Tokens[%d]=%+v want %+v", i, got.Tokens[i], tokens[i])
		}
	}
	wantContent := []ports.Token{
		{Surface: "私", Reading: "わたし", Content: true},
		{Surface: "本", Reading: "ほん", Content: true},
		{Surface: "読む", Reading: "よむ", Content: true},
	}
	if len(got.ContentWords) != len(wantContent) {
		t.Fatalf("ContentWords=%+v want %+v", got.ContentWords, wantContent)
	}
	for i := range wantContent {
		if got.ContentWords[i] != wantContent[i] {
			t.Fatalf("ContentWords[%d]=%+v want %+v", i, got.ContentWords[i], wantContent[i])
		}
	}
}

func TestAnalyzeSentence_ContentWordFilter_OmitsParticlesAndFunction(t *testing.T) {
	// Stub tokens with explicit content vs non-content flags (product rule under test).
	tokens := []ports.Token{
		{Surface: "病院", Reading: "びょういん", Content: true}, // noun
		{Surface: "に", Reading: "", Content: false},      // particle
		{Surface: "行っ", Reading: "いっ", Content: true},    // verb stem-ish
		{Surface: "た", Reading: "", Content: false},      // auxiliary
		{Surface: "。", Reading: "", Content: false},      // punctuation
	}
	m := newApp(t, fakeAnalyzer{byText: map[string][]ports.Token{"病院に行った。": tokens}})

	got, err := m.AnalyzeSentence("病院に行った。")
	if err != nil {
		t.Fatalf("AnalyzeSentence: %v", err)
	}
	for _, w := range got.ContentWords {
		if !w.Content {
			t.Fatalf("non-content token leaked into ContentWords: %+v", w)
		}
		if w.Surface == "に" || w.Surface == "た" || w.Surface == "。" {
			t.Fatalf("particle/function/punct must be omitted: %+v", w)
		}
	}
	if len(got.ContentWords) != 2 {
		t.Fatalf("ContentWords len=%d want 2 (noun+verb only): %+v", len(got.ContentWords), got.ContentWords)
	}
	// Full token stream still includes particles for furigana alignment.
	if len(got.Tokens) != 5 {
		t.Fatalf("Tokens len=%d want 5 (all stubs for furigana)", len(got.Tokens))
	}
}

func TestAnalyzeSentence_AnalyzerError_IsControlledFailure(t *testing.T) {
	m := newApp(t, fakeAnalyzer{failWith: errors.New("engine down")})

	_, err := m.AnalyzeSentence("何か")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, app.ErrAnalyze) {
		t.Fatalf("got %v, want ErrAnalyze", err)
	}
}

func TestAnalyzeSentence_EmptySentence(t *testing.T) {
	m := newApp(t, fakeAnalyzer{})

	_, err := m.AnalyzeSentence("   ")
	if !errors.Is(err, app.ErrEmptySentence) {
		t.Fatalf("got %v, want ErrEmptySentence", err)
	}
}

func TestAnalyzeOnly_LeavesQueueEmpty(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)

	if _, err := m.AnalyzeSentence("私は本を読む。"); err != nil {
		t.Fatal(err)
	}
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("analyze must not write queue; got %d entries", len(list))
	}
}

func TestAddUnknown_FirstSave_CreatesEntryAndFirstUnknownAt(t *testing.T) {
	m := newApp(t, nil)

	res, err := m.AddUnknown("私は本を読む。", "本", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.EntryID == "" {
		t.Fatal("expected entry id")
	}
	if !res.Created || !res.Added || res.Duplicate {
		t.Fatalf("flags: created=%v added=%v dup=%v", res.Created, res.Added, res.Duplicate)
	}
	if len(res.Unknowns) != 1 || res.Unknowns[0] != "本" {
		t.Fatalf("unknowns=%v", res.Unknowns)
	}

	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list len=%d want 1", len(list))
	}
	if list[0].ID != res.EntryID {
		t.Fatalf("list id=%q want %q", list[0].ID, res.EntryID)
	}
	if list[0].FirstUnknownAt.IsZero() {
		t.Fatal("FirstUnknownAt must be set on first save")
	}
	if list[0].Sentence != "私は本を読む。" {
		t.Fatalf("sentence=%q", list[0].Sentence)
	}
}

func TestAddUnknown_DuplicateSurface_SameEntry_NoSecond(t *testing.T) {
	m := newApp(t, nil)

	first, err := m.AddUnknown("病院に行った。", "病院", "", "")
	if err != nil {
		t.Fatal(err)
	}
	firstAt := mustFirstUnknownAt(t, m, first.EntryID)

	dup, err := m.AddUnknown("病院に行った。", "病院", first.EntryID, "")
	if err != nil {
		t.Fatal(err)
	}
	if !dup.Duplicate || dup.Added || dup.Created {
		t.Fatalf("want duplicate only: %+v", dup)
	}
	if len(dup.Unknowns) != 1 {
		t.Fatalf("unknowns=%v want one", dup.Unknowns)
	}
	if mustFirstUnknownAt(t, m, first.EntryID) != firstAt {
		t.Fatal("FirstUnknownAt must not change on duplicate")
	}
}

func TestAddUnknown_SecondDifferentSurface_AppendsOrder(t *testing.T) {
	m := newApp(t, nil)

	first, err := m.AddUnknown("私は本を読む。", "私", "", "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.AddUnknown("私は本を読む。", "本", first.EntryID, "")
	if err != nil {
		t.Fatal(err)
	}
	if second.Duplicate || !second.Added || second.Created {
		t.Fatalf("want append: %+v", second)
	}
	if len(second.Unknowns) != 2 || second.Unknowns[0] != "私" || second.Unknowns[1] != "本" {
		t.Fatalf("order=%v", second.Unknowns)
	}
}

func TestAddUnknown_SameSentenceText_TwoMiningPasses_TwoEntryIDs(t *testing.T) {
	m := newApp(t, nil)
	sentence := "私は本を読む。"

	a, err := m.AddUnknown(sentence, "本", "", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.AddUnknown(sentence, "本", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.EntryID == b.EntryID {
		t.Fatalf("must not merge by text: both id=%q", a.EntryID)
	}
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("list len=%d want 2", len(list))
	}
}

func TestAddUnknown_Persistence_RestartNewAppSameStore(t *testing.T) {
	// Prefer real temp file store (ticket 03 persistence contract).
	path := t.TempDir() + "/queue.json"
	store := mustFileStore(t, path)

	m1 := newAppWithQueue(t, nil, store)
	res, err := m1.AddUnknown("今日は雨だ。", "雨", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m1.AddUnknown("今日は雨だ。", "今日", res.EntryID, ""); err != nil {
		t.Fatal(err)
	}

	// New app instance, same durable store ("restart").
	m2 := newAppWithQueue(t, nil, mustFileStore(t, path))
	list, err := m2.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("after restart len=%d want 1", len(list))
	}
	if list[0].ID != res.EntryID {
		t.Fatalf("id=%q want %q", list[0].ID, res.EntryID)
	}
	if len(list[0].Unknowns) != 2 || list[0].Unknowns[0] != "雨" || list[0].Unknowns[1] != "今日" {
		t.Fatalf("unknowns=%v", list[0].Unknowns)
	}
	if list[0].FirstUnknownAt.IsZero() {
		t.Fatal("FirstUnknownAt lost after restart")
	}
}

func mustFirstUnknownAt(t *testing.T, m *app.MiningApp, id string) time.Time {
	t.Helper()
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range list {
		if e.ID == id {
			return e.FirstUnknownAt
		}
	}
	t.Fatalf("entry %q not found", id)
	return time.Time{}
}

func mustFileStore(t *testing.T, path string) ports.QueueStore {
	t.Helper()
	return queuestore.NewFile(path)
}

func TestAddUnknown_SamePass_ConcurrentFirstTaps_OneEntry(t *testing.T) {
	// Two empty entry_id posts with same pass_id must share one entry (UI multi-tap race).
	m := newApp(t, nil)
	analysis, err := m.AnalyzeSentence("私は本を読む。")
	if err != nil {
		t.Fatal(err)
	}
	pass := analysis.PassID
	if pass == "" {
		t.Fatal("expected PassID from analyze")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	surfaces := []string{"私", "本"}
	for _, s := range surfaces {
		wg.Add(1)
		go func(surface string) {
			defer wg.Done()
			_, err := m.AddUnknown("私は本を読む。", surface, "", pass)
			errCh <- err
		}(s)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("entries=%d want 1 (same pass)", len(list))
	}
	if len(list[0].Unknowns) != 2 {
		t.Fatalf("unknowns=%v want both surfaces", list[0].Unknowns)
	}
	seen := map[string]bool{}
	for _, u := range list[0].Unknowns {
		seen[u] = true
	}
	if !seen["私"] || !seen["本"] {
		t.Fatalf("unknowns missing expected surfaces: %v", list[0].Unknowns)
	}
}

func TestAddUnknown_ConcurrentAppend_DifferentSurfaces_BothKept(t *testing.T) {
	// Prefer real file store: proves AppendUnknown atomic RMW under lock.
	path := t.TempDir() + "/queue.json"
	store := mustFileStore(t, path)
	m := newAppWithQueue(t, nil, store)

	first, err := m.AddUnknown("私は本を読む。", "私", "", "")
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for _, surface := range []string{"本", "読む"} {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			_, err := m.AddUnknown("私は本を読む。", s, first.EntryID, "")
			errCh <- err
		}(surface)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}

	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("entries=%d want 1", len(list))
	}
	if len(list[0].Unknowns) != 3 {
		t.Fatalf("unknowns=%v want 3 (私+本+読む)", list[0].Unknowns)
	}
	seen := map[string]bool{}
	for _, u := range list[0].Unknowns {
		seen[u] = true
	}
	for _, want := range []string{"私", "本", "読む"} {
		if !seen[want] {
			t.Fatalf("missing %q in %v", want, list[0].Unknowns)
		}
	}
}

func TestAddUnknown_EmptySurface_AndMissingEntry(t *testing.T) {
	m := newApp(t, nil)
	if _, err := m.AddUnknown("x", "  ", "", ""); !errors.Is(err, app.ErrEmptySurface) {
		t.Fatalf("empty surface: %v", err)
	}
	if _, err := m.AddUnknown("", "本", "", ""); !errors.Is(err, app.ErrEmptySentence) {
		t.Fatalf("empty sentence create: %v", err)
	}
	if _, err := m.AddUnknown("x", "本", "no-such-id", ""); !errors.Is(err, app.ErrEntryNotFound) {
		t.Fatalf("missing entry: %v", err)
	}
}

func TestExportMarkdown_NestedListShape(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := q.Create(ports.QueueEntry{
		ID: "a", Sentence: "病院に行った。", Unknowns: []string{"病院", "行った"}, FirstUnknownAt: t0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.Create(ports.QueueEntry{
		ID: "b", Sentence: "今日は雨だ。", Unknowns: []string{"雨"}, FirstUnknownAt: t0.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	md, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	want := "- 病院に行った。\n  - 病院\n  - 行った\n- 今日は雨だ。\n  - 雨\n"
	if md != want {
		t.Fatalf("export:\n%s\nwant:\n%s", md, want)
	}
}

func TestExportMarkdown_OrderByFirstUnknownAt_ThenEntryID(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)
	t0 := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	// Insert reverse of expected export order.
	for _, e := range []ports.QueueEntry{
		{ID: "z-late", Sentence: "遅い。", Unknowns: []string{"遅い"}, FirstUnknownAt: t0.Add(2 * time.Minute)},
		{ID: "b-tie", Sentence: "同刻B。", Unknowns: []string{"B"}, FirstUnknownAt: t0},
		{ID: "a-tie", Sentence: "同刻A。", Unknowns: []string{"A"}, FirstUnknownAt: t0},
		{ID: "m-mid", Sentence: "中間。", Unknowns: []string{"中"}, FirstUnknownAt: t0.Add(time.Minute)},
	} {
		if err := q.Create(e); err != nil {
			t.Fatal(err)
		}
	}

	md, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	// Same timestamp: a-tie before b-tie (id ascending). Then mid, then late.
	want := "- 同刻A。\n  - A\n- 同刻B。\n  - B\n- 中間。\n  - 中\n- 遅い。\n  - 遅い\n"
	if md != want {
		t.Fatalf("export:\n%s\nwant:\n%s", md, want)
	}
}

func TestExportMarkdown_UnknownsFirstTapOrder(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)
	if err := q.Create(ports.QueueEntry{
		ID: "e1", Sentence: "私は本を読む。", Unknowns: []string{"私", "本", "読む"},
		FirstUnknownAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	md, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	want := "- 私は本を読む。\n  - 私\n  - 本\n  - 読む\n"
	if md != want {
		t.Fatalf("got %q want %q", md, want)
	}
}

func TestExportMarkdown_NewlineInSentence_Flattened(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)
	if err := q.Create(ports.QueueEntry{
		ID: "e1", Sentence: "一行目\n二行目", Unknowns: []string{"一\n行"},
		FirstUnknownAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	md, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	// No raw newlines inside list items — only structural newlines after each item.
	want := "- 一行目 二行目\n  - 一 行\n"
	if md != want {
		t.Fatalf("got %q want %q", md, want)
	}
	// Exactly two lines of list content (sentence + one unknown) → 2 trailing newlines after items = 2 lines ending with \n
	if strings.Count(md, "\n") != 2 {
		t.Fatalf("newline count=%d body=%q", strings.Count(md, "\n"), md)
	}
}

func TestExportMarkdown_SpecialChars_DoNotBreakListStructure(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)
	// Markdown-ish specials + a fake nested-list line inside sentence (newlines flattened).
	sentence := "# heading *em* **bold** [x](y) `code`\n- fake bullet\n> quote"
	surface := "*表面* #1 [a](b)"
	if err := q.Create(ports.QueueEntry{
		ID: "e1", Sentence: sentence, Unknowns: []string{surface, "普通"},
		FirstUnknownAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	md, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(md, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 list lines (1 sentence + 2 unknowns), got %d:\n%s", len(lines), md)
	}
	if !strings.HasPrefix(lines[0], "- ") {
		t.Fatalf("top-level item: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "  - ") || !strings.HasPrefix(lines[2], "  - ") {
		t.Fatalf("nested items: %q / %q", lines[1], lines[2])
	}
	// Fake bullet from sentence must not become its own top-level list row.
	topLevel := 0
	for _, ln := range lines {
		if strings.HasPrefix(ln, "- ") && !strings.HasPrefix(ln, "  - ") {
			topLevel++
		}
	}
	if topLevel != 1 {
		t.Fatalf("top-level count=%d want 1 body=%q", topLevel, md)
	}
	if !strings.Contains(lines[0], "# heading") || !strings.Contains(lines[0], "fake bullet") {
		t.Fatalf("sentence content lost: %q", lines[0])
	}
	if lines[1] != "  - "+surface {
		t.Fatalf("unknown0=%q", lines[1])
	}
	if lines[2] != "  - 普通" {
		t.Fatalf("unknown1=%q", lines[2])
	}
}

func TestExportMarkdown_SameSentenceText_TwoEntries(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sentence := "同じ文。"
	if err := q.Create(ports.QueueEntry{
		ID: "e1", Sentence: sentence, Unknowns: []string{"同"}, FirstUnknownAt: t0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.Create(ports.QueueEntry{
		ID: "e2", Sentence: sentence, Unknowns: []string{"文"}, FirstUnknownAt: t0.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	md, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	want := "- 同じ文。\n  - 同\n- 同じ文。\n  - 文\n"
	if md != want {
		t.Fatalf("got %q want %q", md, want)
	}
}

func TestExportMarkdown_EmptyQueue_EmptyDocument(t *testing.T) {
	m := newApp(t, nil)
	md, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	if md != "" {
		t.Fatalf("want empty document, got %q", md)
	}
}

func TestExportMarkdown_SkipsZeroUnknownEntries_LeavesStoreUnchanged(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := q.Create(ports.QueueEntry{
		ID: "empty", Sentence: "空。", Unknowns: []string{}, FirstUnknownAt: t0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.Create(ports.QueueEntry{
		ID: "ok", Sentence: "有る。", Unknowns: []string{"有"}, FirstUnknownAt: t0.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	before, err := q.List()
	if err != nil {
		t.Fatal(err)
	}
	md, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	if md != "- 有る。\n  - 有\n" {
		t.Fatalf("md=%q", md)
	}
	// Re-export same
	md2, err := m.ExportMarkdown()
	if err != nil {
		t.Fatal(err)
	}
	if md2 != md {
		t.Fatal("re-export must match")
	}
	after, err := q.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("store mutated: before=%d after=%d", len(before), len(after))
	}
	if len(after) != 2 {
		t.Fatalf("want 2 entries still, got %d", len(after))
	}
}

func TestClearAll_EmptiesStore_AndNoOpWhenEmpty(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)
	if _, err := m.AddUnknown("病院に行った。", "病院", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := m.AddUnknown("今日は雨だ。", "雨", "", ""); err != nil {
		t.Fatal(err)
	}
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("setup entries=%d", len(list))
	}

	if err := m.ClearAll(); err != nil {
		t.Fatal(err)
	}
	list, err = m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("after clear entries=%d", len(list))
	}
	md, err := m.ExportMarkdown()
	if err != nil || md != "" {
		t.Fatalf("export after clear: md=%q err=%v", md, err)
	}

	// Second clear is no-op
	if err := m.ClearAll(); err != nil {
		t.Fatal(err)
	}
	list, err = m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("second clear entries=%d", len(list))
	}
}

func TestIngestPage_OCRTextToCandidates(t *testing.T) {
	m := newAppWithOCR(t, ocr.Stub{Text: "病院に行った。今日は雨だ。"})
	img := []byte("fake-png-bytes")
	got, err := m.IngestPage(img)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "病院に行った。今日は雨だ。" {
		t.Fatalf("Text=%q", got.Text)
	}
	if len(got.Candidates) != 2 {
		t.Fatalf("Candidates=%#v", got.Candidates)
	}
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("ingest must not write queue; got %d", len(list))
	}
}

func TestIngestPage_OversizeRejected(t *testing.T) {
	m := newAppWithOCR(t, ocr.Stub{Text: "should-not-run"})
	img := make([]byte, app.MaxUploadBytes+1)
	_, err := m.IngestPage(img)
	if !errors.Is(err, app.ErrPayloadTooLarge) {
		t.Fatalf("got %v want ErrPayloadTooLarge", err)
	}
}

func TestIngestPage_EmptyImage(t *testing.T) {
	m := newAppWithOCR(t, ocr.Stub{Text: "x"})
	_, err := m.IngestPage(nil)
	if !errors.Is(err, app.ErrEmptyPage) {
		t.Fatalf("got %v want ErrEmptyPage", err)
	}
}

func TestIngestPage_OCRFailure(t *testing.T) {
	m := newAppWithOCR(t, ocr.Stub{FailWith: errors.New("engine down")})
	_, err := m.IngestPage([]byte("img"))
	if !errors.Is(err, app.ErrOcrFailed) {
		t.Fatalf("got %v want ErrOcrFailed", err)
	}
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("queue should stay empty after OCR fail; got %d", len(list))
	}
}

func TestIngestPage_EmptyOCRText(t *testing.T) {
	m := newAppWithOCR(t, ocr.Stub{Text: "   "})
	_, err := m.IngestPage([]byte("img"))
	if !errors.Is(err, app.ErrEmptyPage) {
		t.Fatalf("got %v want ErrEmptyPage", err)
	}
}

func TestIngestPage_SingleFlight(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	engine := &slowOCR{
		started: started,
		release: release,
		text:    "私は本を読む。",
	}
	m := newAppWithOCR(t, engine)

	errCh := make(chan error, 1)
	go func() {
		_, err := m.IngestPage([]byte("img-a"))
		errCh <- err
	}()
	<-started

	_, err := m.IngestPage([]byte("img-b"))
	if !errors.Is(err, app.ErrIngestBusy) {
		t.Fatalf("second ingest: %v want ErrIngestBusy", err)
	}
	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("first ingest: %v", err)
	}
}

func TestIngestPage_DoesNotClearQueue(t *testing.T) {
	q := queuestore.NewMem()
	if err := q.Create(ports.QueueEntry{
		ID: "keep", Sentence: "s", Unknowns: []string{"A"}, FirstUnknownAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	m := app.NewMiningApp(pinauth.Static{Secret: "test-pin-ok"}, fakeAnalyzer{}, q, ocr.Stub{Text: "今日は雨だ。"})
	if _, err := m.IngestPage([]byte("img")); err != nil {
		t.Fatal(err)
	}
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "keep" {
		t.Fatalf("queue corrupted: %+v", list)
	}
}

// slowOCR blocks after signaling started until release is closed.
type slowOCR struct {
	started chan struct{}
	release chan struct{}
	text    string
	once    sync.Once
}

func (s *slowOCR) Recognize(image []byte) (string, error) {
	s.once.Do(func() { close(s.started) })
	<-s.release
	return s.text, nil
}

func TestIngestPage_ExactMaxUploadBytesAllowed(t *testing.T) {
	m := newAppWithOCR(t, ocr.Stub{Text: "私は本を読む。"})
	img := make([]byte, app.MaxUploadBytes)
	got, err := m.IngestPage(img)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 1 {
		t.Fatalf("candidates=%#v", got.Candidates)
	}
}

func TestNewMiningApp_RequiresPorts(t *testing.T) {
	q := queuestore.NewMem()
	a := fakeAnalyzer{}
	o := ocr.Stub{Text: "x"}
	p := pinauth.Static{Secret: "p"}

	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s: expected panic", name)
			}
		}()
		fn()
	}
	mustPanic("nil pin", func() { app.NewMiningApp(nil, a, q, o) })
	mustPanic("nil analyzer", func() { app.NewMiningApp(p, nil, q, o) })
	mustPanic("nil queue", func() { app.NewMiningApp(p, a, nil, o) })
	mustPanic("nil ocr", func() { app.NewMiningApp(p, a, q, nil) })
}
