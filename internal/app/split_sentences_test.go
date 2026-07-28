package app_test

import (
	"reflect"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/ports"
)

func TestSplitSentences_MultiSentenceFixture(t *testing.T) {
	in := "病院に行った。今日は雨だ。私は本を読む。"
	want := []string{
		"病院に行った。",
		"今日は雨だ。",
		"私は本を読む。",
	}
	got := app.SplitSentences(in)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SplitSentences=%#v want %#v", got, want)
	}
}

func TestSplitSentences_FullwidthAndHalfwidthTerminators(t *testing.T) {
	in := "本当か？信じられない！彼は笑った．Yes!"
	got := app.SplitSentences(in)
	want := []string{
		"本当か？",
		"信じられない！",
		"彼は笑った．",
		"Yes!",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestSplitSentences_NoTerminator_SingleBlob(t *testing.T) {
	in := "終止符のない長い文"
	got := app.SplitSentences(in)
	want := []string{in}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestSplitSentences_Empty_DocumentedEmpty(t *testing.T) {
	if got := app.SplitSentences(""); got != nil {
		t.Fatalf("empty: got %#v want nil", got)
	}
	if got := app.SplitSentences("   \n\t  "); got != nil {
		t.Fatalf("whitespace: got %#v want nil", got)
	}
}

func TestSplitSentences_TrailingWithoutTerminator(t *testing.T) {
	in := "第一文。第二文は終わりなし"
	want := []string{"第一文。", "第二文は終わりなし"}
	got := app.SplitSentences(in)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestSplitSentences_NewlinesCollapsedIntoCandidates(t *testing.T) {
	// Newlines are not terminators; they stay inside a candidate until 。
	in := "一行目です。\n二行目です。"
	got := app.SplitSentences(in)
	if len(got) != 2 {
		t.Fatalf("len=%d got %#v", len(got), got)
	}
	if got[0] != "一行目です。" {
		t.Fatalf("got[0]=%q", got[0])
	}
	if got[1] != "二行目です。" {
		t.Fatalf("got[1]=%q", got[1])
	}
}

func TestProposeSentences_DoesNotWriteQueue(t *testing.T) {
	q := queuestore.NewMem()
	m := newAppWithQueue(t, nil, q)

	cands := m.ProposeSentences("病院に行った。今日は雨だ。")
	if len(cands) != 2 {
		t.Fatalf("candidates=%#v", cands)
	}
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("propose must not write queue; got %d", len(list))
	}
}

func TestSelectCandidate_Analyze_DoesNotCreateQueueUntilUnknown(t *testing.T) {
	sentence := "私は本を読む。"
	tokens := []ports.Token{
		{Surface: "私", Reading: "わたし", Content: true},
		{Surface: "は", Reading: "", Content: false},
		{Surface: "本", Reading: "ほん", Content: true},
		{Surface: "を", Reading: "", Content: false},
		{Surface: "読む", Reading: "よむ", Content: true},
		{Surface: "。", Reading: "", Content: false},
	}
	q := queuestore.NewMem()
	m := newAppWithQueue(t, fakeAnalyzer{byText: map[string][]ports.Token{sentence: tokens}}, q)

	page := "病院に行った。" + sentence
	cands := m.ProposeSentences(page)
	if len(cands) < 2 {
		t.Fatalf("cands=%#v", cands)
	}
	// Learner picks last candidate (= working sentence) and analyzes.
	got, err := m.AnalyzeSentence(cands[len(cands)-1])
	if err != nil {
		t.Fatal(err)
	}
	if got.Sentence != sentence {
		t.Fatalf("Sentence=%q", got.Sentence)
	}
	if len(got.ContentWords) != 3 {
		t.Fatalf("ContentWords=%+v", got.ContentWords)
	}
	list, err := m.ListQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("select+analyze must not write queue: %d", len(list))
	}
}

func TestEditWorkingSentence_ReAnalyze_NewTokens(t *testing.T) {
	m := newApp(t, fakeAnalyzer{
		byText: map[string][]ports.Token{
			"病院に行った。": {
				{Surface: "病院", Reading: "びょういん", Content: true},
				{Surface: "に", Reading: "", Content: false},
				{Surface: "行っ", Reading: "いっ", Content: true},
				{Surface: "た", Reading: "", Content: false},
				{Surface: "。", Reading: "", Content: false},
			},
			"今日は雨だ。": {
				{Surface: "今日", Reading: "きょう", Content: true},
				{Surface: "は", Reading: "", Content: false},
				{Surface: "雨", Reading: "あめ", Content: true},
				{Surface: "だ", Reading: "", Content: false},
				{Surface: "。", Reading: "", Content: false},
			},
		},
	})

	first, err := m.AnalyzeSentence("病院に行った。")
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentWords[0].Surface != "病院" {
		t.Fatalf("first content=%+v", first.ContentWords)
	}

	// Edit working sentence and re-analyze (product: edited string is canonical).
	second, err := m.AnalyzeSentence("今日は雨だ。")
	if err != nil {
		t.Fatal(err)
	}
	if second.ContentWords[0].Surface != "今日" {
		t.Fatalf("edited content=%+v", second.ContentWords)
	}
	if first.PassID == second.PassID {
		t.Fatal("re-analyze must issue new PassID")
	}
}
