package app_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/ports"
)

// fakePinAuth is a test double for ports.PinAuth. It does not use production secrets.
type fakePinAuth struct {
	valid string
}

func (f fakePinAuth) Verify(pin string) bool {
	return pin == f.valid
}

// fakeAnalyzer is a test double for ports.JapaneseAnalyzer.
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

type memQueue struct {
	mu    sync.Mutex
	byID  map[string]ports.QueueEntry
	order []string
}

func newMemQueue() *memQueue {
	return &memQueue{byID: map[string]ports.QueueEntry{}}
}

func (m *memQueue) Create(entry ports.QueueEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[entry.ID]; ok {
		return errors.New("duplicate id")
	}
	cp := entry
	cp.Unknowns = append([]string(nil), entry.Unknowns...)
	m.byID[entry.ID] = cp
	m.order = append(m.order, entry.ID)
	return nil
}

func (m *memQueue) Update(entry ports.QueueEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.byID[entry.ID]; !ok {
		return errors.New("missing id")
	}
	cp := entry
	cp.Unknowns = append([]string(nil), entry.Unknowns...)
	m.byID[entry.ID] = cp
	return nil
}

func (m *memQueue) Get(id string) (ports.QueueEntry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.byID[id]
	if !ok {
		return ports.QueueEntry{}, false, nil
	}
	cp := e
	cp.Unknowns = append([]string(nil), e.Unknowns...)
	return cp, true, nil
}

func (m *memQueue) List() ([]ports.QueueEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ports.QueueEntry, 0, len(m.order))
	for _, id := range m.order {
		e := m.byID[id]
		cp := e
		cp.Unknowns = append([]string(nil), e.Unknowns...)
		out = append(out, cp)
	}
	return out, nil
}

func (m *memQueue) AppendUnknown(id, surface string) (ports.QueueEntry, bool, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.byID[id]
	if !ok {
		return ports.QueueEntry{}, false, false, nil
	}
	for _, u := range e.Unknowns {
		if u == surface {
			cp := e
			cp.Unknowns = append([]string(nil), e.Unknowns...)
			return cp, false, true, nil
		}
	}
	e.Unknowns = append(append([]string(nil), e.Unknowns...), surface)
	m.byID[id] = e
	cp := e
	cp.Unknowns = append([]string(nil), e.Unknowns...)
	return cp, true, true, nil
}

func newApp(t *testing.T, analyzer ports.JapaneseAnalyzer) *app.MiningApp {
	t.Helper()
	return newAppWithQueue(t, analyzer, newMemQueue())
}

func newAppWithQueue(t *testing.T, analyzer ports.JapaneseAnalyzer, queue ports.QueueStore) *app.MiningApp {
	t.Helper()
	if analyzer == nil {
		analyzer = fakeAnalyzer{}
	}
	return app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"}, analyzer, queue)
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
		{Surface: "に", Reading: "", Content: false},          // particle
		{Surface: "行っ", Reading: "いっ", Content: true},     // verb stem-ish
		{Surface: "た", Reading: "", Content: false},          // auxiliary
		{Surface: "。", Reading: "", Content: false},          // punctuation
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
	q := newMemQueue()
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
