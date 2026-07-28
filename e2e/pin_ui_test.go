package e2e_test

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
	"github.com/rikiisworking/miner/internal/adapters/ocr"
	"github.com/rikiisworking/miner/internal/adapters/pinauth"
	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/httpapi"
	"github.com/rikiisworking/miner/internal/ocrtest"
	"github.com/rikiisworking/miner/internal/ports"
	"github.com/rikiisworking/miner/web"
)

// defaultE2EOCR: multi-sentence text so photo + page-text journeys work without
// host NDLOCR-Lite. Real engine only where a test calls startServerWith + MustEngine.
var defaultE2EOCR ports.OcrEngine = ocr.Static{Text: "病院に行った。\n私は本を読む。"}

func startServer(t *testing.T) (baseURL string, shutdown func()) {
	t.Helper()
	return startServerWith(t, defaultE2EOCR)
}

func startServerWith(t *testing.T, eng ports.OcrEngine) (baseURL string, shutdown func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	queuePath := filepath.Join(t.TempDir(), "queue.json")
	m := app.NewMiningApp(
		pinauth.Static{Secret: "test-pin-ok"},
		analyzer.Stub{},
		queuestore.NewFile(queuePath),
		eng,
	)
	s, err := httpapi.New(httpapi.Config{
		MiningApp: m,
		WebFS:     web.FS(),
		Addr:      ln.Addr().String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = s.App().Listener(ln)
		close(done)
	}()
	baseURL = fmt.Sprintf("http://%s", ln.Addr().String())
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", ln.Addr().String(), 50*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return baseURL, func() {
		_ = s.Shutdown()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func newBrowser(t *testing.T) *rod.Browser {
	t.Helper()
	l := launcher.New().
		Headless(true).
		Set("no-sandbox").
		Set("disable-gpu").
		Set("disable-dev-shm-usage")
	u, err := l.Launch()
	if err != nil {
		t.Fatalf("launch browser: %v", err)
	}
	b := rod.New().ControlURL(u).Timeout(20 * time.Second)
	if err := b.Connect(); err != nil {
		t.Fatalf("connect browser: %v", err)
	}
	t.Cleanup(func() {
		_ = b.Close()
		l.Cleanup()
	})
	return b
}

func openPINPage(t *testing.T, browser *rod.Browser, base string) *rod.Page {
	t.Helper()
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	// No WaitLoad — local embedded assets only.
	if err := page.Timeout(15 * time.Second).Navigate(base + "/"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="pin-input"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("pin-input not found: %v\nhtml=%s", err, html)
	}
	return page
}

func fillPIN(t *testing.T, page *rod.Page, pin string) {
	t.Helper()
	input, err := page.Timeout(5 * time.Second).Element(`[data-testid="pin-input"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := input.SelectAllText(); err != nil {
		t.Fatal(err)
	}
	if err := input.Input(pin); err != nil {
		t.Fatal(err)
	}
	btn, err := page.Timeout(5 * time.Second).Element(`[data-testid="pin-submit"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
}

func unlockToShell(t *testing.T, browser *rod.Browser, base string) *rod.Page {
	t.Helper()
	page := openPINPage(t, browser, base)
	fillPIN(t, page, "test-pin-ok")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="app-shell"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("app-shell missing: %v\nhtml=%s", err, html)
	}
	return page
}

func TestUI_WrongPIN_ShowsError_NoShell(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := openPINPage(t, browser, base)
	fillPIN(t, page, "wrong-pin")

	el, err := page.Timeout(10 * time.Second).Element(`[data-testid="pin-error"]`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("pin-error missing: %v\nhtml=%s", err, html)
	}
	if txt, _ := el.Text(); txt == "" {
		t.Fatal("expected pin error text")
	}
	has, _, _ := page.Has(`[data-testid="app-shell"]`)
	if has {
		t.Fatal("app shell must not show after wrong PIN")
	}
}

func TestUI_CorrectPIN_ShowsShell(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := openPINPage(t, browser, base)
	fillPIN(t, page, "test-pin-ok")

	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="app-shell"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("app-shell missing: %v\nhtml=%s", err, html)
	}
	ready, err := page.Timeout(5 * time.Second).Element(`[data-testid="shell-ready"]`)
	if err != nil {
		t.Fatalf("shell-ready missing: %v", err)
	}
	if s, _ := ready.Text(); s == "" {
		t.Fatal("expected shell ready text")
	}
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="sentence-input"]`); err != nil {
		t.Fatalf("sentence-input missing after unlock: %v", err)
	}
}

func setSentence(t *testing.T, page *rod.Page, text string) {
	t.Helper()
	// Set value via DOM so special/underscore strings are reliable under headless rod.
	_, err := page.Eval(`(t) => {
		const el = document.querySelector('[data-testid="sentence-input"]');
		if (!el) throw new Error('sentence-input missing');
		el.focus();
		el.value = t;
		el.dispatchEvent(new Event('input', { bubbles: true }));
		el.dispatchEvent(new Event('change', { bubbles: true }));
	}`, text)
	if err != nil {
		t.Fatalf("set sentence: %v", err)
	}
}

func submitAnalyze(t *testing.T, page *rod.Page) {
	t.Helper()
	btn, err := page.Timeout(5 * time.Second).Element(`[data-testid="analyze-submit"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
}

func TestUI_Analyze_PasteFixture_RubyAndContentWords(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	setSentence(t, page, "私は本を読む。")
	submitAnalyze(t, page)

	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("analyze-success missing: %v\nhtml=%s", err, html)
	}
	// HTML ruby furigana present
	ruby, err := page.Timeout(5 * time.Second).Element(`ruby[data-testid="ruby-token"]`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("ruby token missing: %v\nhtml=%s", err, html)
	}
	if txt, _ := ruby.Text(); txt == "" {
		t.Fatal("ruby token empty")
	}
	if _, err := page.Element(`rt`); err != nil {
		t.Fatalf("rt (reading) missing: %v", err)
	}

	words, err := page.Elements(`[data-testid="content-word"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 3 {
		html, _ := page.HTML()
		t.Fatalf("content-word count=%d want 3\nhtml=%s", len(words), html)
	}
	// No bare particle-only content rows for fixture
	for _, w := range words {
		surface, err := w.Attribute("data-surface")
		if err != nil || surface == nil {
			t.Fatalf("content-word missing data-surface: %v", err)
		}
		if *surface == "は" || *surface == "を" || *surface == "。" {
			t.Fatalf("particle/punct must not be content-word row: %s", *surface)
		}
	}
}

func TestUI_Analyze_ForceError_ShowsMessage(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	setSentence(t, page, analyzer.ForceErrorText)
	submitAnalyze(t, page)

	el, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-error"]`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("analyze-error missing: %v\nhtml=%s", err, html)
	}
	txt, _ := el.Text()
	if txt == "" {
		t.Fatal("expected analyze error text")
	}
	// Phone-readable: explain failure + next step.
	if !strings.Contains(txt, "could not be tokenized") || !strings.Contains(strings.ToLower(txt), "edit") {
		t.Fatalf("analyze error not actionable enough: %q", txt)
	}
	has, _, _ := page.Has(`[data-testid="analyze-success"]`)
	if has {
		t.Fatal("analyze-success must not show on forced error")
	}
}

func TestUI_Queue_EmptyState_Visible(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	nav, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := nav.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-page"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-page missing: %v\nhtml=%s", err, html)
	}
	empty, err := page.Timeout(5 * time.Second).Element(`[data-testid="queue-empty"]`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-empty missing: %v\nhtml=%s", err, html)
	}
	if txt, _ := empty.Text(); !strings.Contains(strings.ToLower(txt), "empty") {
		t.Fatalf("empty copy=%q", txt)
	}
	clearBtn, err := page.Timeout(5 * time.Second).Element(`[data-testid="clear-all"]`)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := clearBtn.Property("disabled")
	if err != nil {
		t.Fatal(err)
	}
	if !disabled.Bool() {
		t.Fatal("clear-all should be disabled on empty queue")
	}
	// Export still available when empty
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="export-markdown"]`); err != nil {
		t.Fatal(err)
	}
}

func clickMarkUnknown(t *testing.T, page *rod.Page, surface string) {
	t.Helper()
	sel := fmt.Sprintf(`[data-testid="mark-unknown"][data-surface="%s"]`, surface)
	btn, err := page.Timeout(5 * time.Second).Element(sel)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("mark-unknown %q missing: %v\nhtml=%s", surface, err, html)
	}
	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
}

func TestUI_MarkUnknown_SaveFeedback_AndQueue(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	setSentence(t, page, "私は本を読む。")
	submitAnalyze(t, page)
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("analyze-success missing: %v\nhtml=%s", err, html)
	}

	clickMarkUnknown(t, page, "本")

	fb, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("save feedback missing: %v\nhtml=%s", err, html)
	}
	if txt, _ := fb.Text(); !strings.Contains(txt, "本") {
		t.Fatalf("feedback text=%q", txt)
	}

	// Open queue
	nav, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := nav.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-page"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-page missing: %v\nhtml=%s", err, html)
	}
	entry, err := page.Timeout(5 * time.Second).Element(`[data-testid="queue-entry"]`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-entry missing: %v\nhtml=%s", err, html)
	}
	if txt, _ := entry.Text(); !strings.Contains(txt, "私は本を読む。") || !strings.Contains(txt, "本") {
		t.Fatalf("queue entry text=%q", txt)
	}
	unk, err := page.Elements(`[data-testid="queue-unknown"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(unk) != 1 {
		t.Fatalf("queue-unknown count=%d want 1", len(unk))
	}
}

func TestUI_MarkUnknown_DuplicateFeedback_CountUnchanged(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	setSentence(t, page, "病院に行った。")
	submitAnalyze(t, page)
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		t.Fatal(err)
	}

	clickMarkUnknown(t, page, "病院")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("save feedback: %v\nhtml=%s", err, html)
	}

	// Wait for OOB entry_id update so second tap appends same entry.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		el, err := page.Element(`[data-testid="entry-id"]`)
		if err == nil {
			v, err := el.Property("value")
			if err == nil && v.Str() != "" {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
	}

	clickMarkUnknown(t, page, "病院")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="duplicate"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("duplicate feedback: %v\nhtml=%s", err, html)
	}

	// Queue still one unknown
	nav, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := nav.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-page"]`); err != nil {
		t.Fatal(err)
	}
	unk, err := page.Elements(`[data-testid="queue-unknown"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(unk) != 1 {
		html, _ := page.HTML()
		t.Fatalf("after dup count=%d want 1\nhtml=%s", len(unk), html)
	}
}

func TestUI_ExportAndClearAll_FullTextPath(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	setSentence(t, page, "病院に行った。")
	submitAnalyze(t, page)
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		t.Fatal(err)
	}
	clickMarkUnknown(t, page, "病院")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("save feedback: %v\nhtml=%s", err, html)
	}

	// Queue page
	nav, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := nav.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-page"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="queue-entry"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue entry missing: %v\nhtml=%s", err, html)
	}

	// Export control present + clickable; fetch markdown via same-origin href
	exportLink, err := page.Timeout(5 * time.Second).Element(`[data-testid="export-markdown"]`)
	if err != nil {
		t.Fatal(err)
	}
	href, err := exportLink.Attribute("href")
	if err != nil || href == nil || *href == "" {
		t.Fatalf("export href missing: %v %v", href, err)
	}

	// Download via page evaluate fetch (cookie session attached)
	result, err := page.Eval(`async (url) => {
		const r = await fetch(url, { credentials: 'same-origin' });
		const text = await r.text();
		return { status: r.status, type: r.headers.get('content-type') || '', text: text };
	}`, *href)
	if err != nil {
		t.Fatalf("export fetch: %v", err)
	}
	status := int(result.Value.Get("status").Num())
	ctype := result.Value.Get("type").Str()
	md := result.Value.Get("text").Str()
	if status != 200 {
		t.Fatalf("export status=%d", status)
	}
	if !strings.Contains(ctype, "markdown") {
		t.Fatalf("content-type=%q", ctype)
	}
	if !strings.Contains(md, "- 病院に行った。") || !strings.Contains(md, "  - 病院") {
		t.Fatalf("export body=%q", md)
	}

	// Queue still has entry after export
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="queue-entry"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue empty after export: %v\nhtml=%s", err, html)
	}

	// Clear all with dialog confirm
	waitDialog, handleDialog := page.MustHandleDialog()
	btn, err := page.Timeout(5 * time.Second).Element(`[data-testid="clear-all"]`)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = btn.Click(proto.InputMouseButtonLeft, 1)
	}()
	waitDialog()
	handleDialog(true, "")

	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-empty"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-empty missing after clear: %v\nhtml=%s", err, html)
	}
	entries, err := page.Elements(`[data-testid="queue-entry"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries after clear=%d", len(entries))
	}

	// Clear all disabled when empty
	clearBtn, err := page.Timeout(5 * time.Second).Element(`[data-testid="clear-all"]`)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := clearBtn.Property("disabled")
	if err != nil {
		t.Fatal(err)
	}
	if !disabled.Bool() {
		t.Fatal("clear-all should be disabled when queue empty")
	}

	// Empty export still works
	result2, err := page.Eval(`async (url) => {
		const r = await fetch(url, { credentials: 'same-origin' });
		const text = await r.text();
		return { status: r.status, text: text };
	}`, *href)
	if err != nil {
		t.Fatal(err)
	}
	if int(result2.Value.Get("status").Num()) != 200 {
		t.Fatalf("empty export status=%v", result2.Value.Get("status"))
	}
	if result2.Value.Get("text").Str() != "" {
		t.Fatalf("empty export body=%q", result2.Value.Get("text").Str())
	}
}

func TestUI_PageText_ProposePickAnalyze_EditReanalyze(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	// Paste multi-sentence page
	_, err := page.Eval(`(t) => {
		const el = document.querySelector('[data-testid="page-text-input"]');
		if (!el) throw new Error('page-text-input missing');
		el.focus();
		el.value = t;
		el.dispatchEvent(new Event('input', { bubbles: true }));
		el.dispatchEvent(new Event('change', { bubbles: true }));
	}`, "病院に行った。今日は雨だ。私は本を読む。")
	if err != nil {
		t.Fatalf("set page text: %v", err)
	}
	submit, err := page.Timeout(5 * time.Second).Element(`[data-testid="page-text-submit"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := submit.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}

	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="sentence-candidates"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("candidates missing: %v\nhtml=%s", err, html)
	}
	cands, err := page.Elements(`[data-testid="sentence-candidate"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 3 {
		html, _ := page.HTML()
		t.Fatalf("candidate count=%d want 3\nhtml=%s", len(cands), html)
	}

	// Pick third candidate (私は本を読む。)
	pick, err := page.Timeout(5 * time.Second).Element(`[data-testid="candidate-pick"][data-index="2"]`)
	if err != nil {
		// fallback: last pick button
		picks, err2 := page.Elements(`[data-testid="candidate-pick"]`)
		if err2 != nil || len(picks) < 3 {
			html, _ := page.HTML()
			t.Fatalf("candidate-pick missing: %v\nhtml=%s", err, html)
		}
		pick = picks[2]
	}
	if err := pick.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}

	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("analyze-success after pick: %v\nhtml=%s", err, html)
	}
	if _, err := page.Timeout(5 * time.Second).Element(`ruby[data-testid="ruby-token"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("furigana after pick: %v\nhtml=%s", err, html)
	}
	// Working sentence box should hold selected text (OOB)
	deadline := time.Now().Add(3 * time.Second)
	var sentenceVal string
	for time.Now().Before(deadline) {
		el, err := page.Element(`[data-testid="sentence-input"]`)
		if err == nil {
			v, err := el.Property("value")
			if err == nil {
				sentenceVal = v.Str()
				if strings.Contains(sentenceVal, "私は本を読む") {
					break
				}
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	if !strings.Contains(sentenceVal, "私は本を読む") {
		t.Fatalf("sentence-input after pick=%q", sentenceVal)
	}

	// Edit working sentence → re-analyze
	setSentence(t, page, "病院に行った。")
	submitAnalyze(t, page)
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("re-analyze success: %v\nhtml=%s", err, html)
	}
	// Content list should include 病院 from edited sentence fixture
	words, err := page.Elements(`[data-testid="content-word"]`)
	if err != nil {
		t.Fatal(err)
	}
	foundByouin := false
	for _, w := range words {
		surface, err := w.Attribute("data-surface")
		if err == nil && surface != nil && *surface == "病院" {
			foundByouin = true
			break
		}
	}
	if !foundByouin {
		html, _ := page.HTML()
		t.Fatalf("edited analyze missing 病院 content-word\nhtml=%s", html)
	}
}

func fixtureImagePath(t *testing.T, caseID string) string {
	t.Helper()
	manifest, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	return manifest.Must(caseID).Path()
}

func TestUI_CameraCapture_ControlPresent_FallbackOrClickable(t *testing.T) {
	// Headless CI has no webcam. Assert camera control + upload still usable.
	// Either getUserMedia works (rare in CI) or fallback/error path appears.
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="camera-section"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("camera-section missing: %v\nhtml=%s", err, html)
	}
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-upload-section"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("photo-upload-section missing next to camera: %v\nhtml=%s", err, html)
	}

	start, err := page.Timeout(5 * time.Second).Element(`[data-testid="camera-start"]`)
	if err != nil {
		t.Fatalf("camera-start missing: %v", err)
	}
	// Control must be clickable (not permanently removed).
	if err := start.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatalf("camera-start not clickable: %v", err)
	}

	// After click: live preview, error, or fallback (no real camera in CI).
	// getUserMedia may hang until the page's 4s open-timeout fires.
	deadline := time.Now().Add(8 * time.Second)
	var settled bool
	for time.Now().Before(deadline) {
		res, err := page.Eval(`() => {
			const visible = (sel) => {
				const el = document.querySelector(sel);
				return !!(el && !el.hidden);
			};
			const err = document.querySelector('[data-testid="camera-error"]');
			const errText = !!(err && !err.hidden && (err.textContent || '').trim());
			return visible('[data-testid="camera-live"]')
				|| errText
				|| visible('[data-testid="camera-fallback"]');
		}`)
		if err == nil && res.Value.Bool() {
			settled = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !settled {
		html, _ := page.HTML()
		t.Fatalf("after camera-start: want live preview, error, or fallback\nhtml=%s", html)
	}

	// File upload still present after camera attempt.
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-input"]`); err != nil {
		t.Fatalf("photo-input missing after camera attempt: %v", err)
	}
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-submit"]`); err != nil {
		t.Fatalf("photo-submit missing after camera attempt: %v", err)
	}
}

func TestUI_PhotoIngest_UploadPickAnalyze_MarkExport(t *testing.T) {
	// Static OCR returns multi-sentence text for any upload (no host NDLOCR-Lite).
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	imgPath := fixtureImagePath(t, "02_multi_sentence")

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-upload-section"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("photo-upload-section missing: %v\nhtml=%s", err, html)
	}
	// Ticket 07: camera control coexists with file upload.
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="camera-section"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("camera-section missing beside upload: %v\nhtml=%s", err, html)
	}

	input, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-input"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := input.SetFiles([]string{imgPath}); err != nil {
		t.Fatalf("set photo file: %v", err)
	}

	submit, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-submit"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := submit.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}

	if _, err := page.Timeout(15 * time.Second).Element(`[data-testid="sentence-candidates"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("candidates after photo: %v\nhtml=%s", err, html)
	}
	cands, err := page.Elements(`[data-testid="sentence-candidate"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) < 2 {
		html, _ := page.HTML()
		t.Fatalf("candidate count=%d want ≥2\nhtml=%s", len(cands), html)
	}

	// Upload control re-enabled after response (hx-disabled-elt).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		btn, err := page.Element(`[data-testid="photo-submit"]`)
		if err == nil {
			dis, err := btn.Property("disabled")
			if err == nil && !dis.Bool() {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
	}

	// Pick first candidate → analyze
	pick, err := page.Timeout(5 * time.Second).Element(`[data-testid="candidate-pick"][data-index="0"]`)
	if err != nil {
		picks, err2 := page.Elements(`[data-testid="candidate-pick"]`)
		if err2 != nil || len(picks) == 0 {
			html, _ := page.HTML()
			t.Fatalf("candidate-pick missing: %v\nhtml=%s", err, html)
		}
		pick = picks[0]
	}
	if err := pick.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("analyze after photo pick: %v\nhtml=%s", err, html)
	}

	// Mark first available content word (stub: 病院 for first sentence of multi fixture)
	words, err := page.Elements(`[data-testid="mark-unknown"]`)
	if err != nil || len(words) == 0 {
		html, _ := page.HTML()
		t.Fatalf("no mark-unknown after photo path\nhtml=%s", html)
	}
	surfaceAttr, err := words[0].Attribute("data-surface")
	if err != nil || surfaceAttr == nil || *surfaceAttr == "" {
		t.Fatal("mark-unknown missing data-surface")
	}
	surface := *surfaceAttr
	if err := words[0].Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("save feedback after photo path: %v\nhtml=%s", err, html)
	}

	// Queue + export still work
	nav, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := nav.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-entry"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-entry after photo path: %v\nhtml=%s", err, html)
	}
	exportLink, err := page.Timeout(5 * time.Second).Element(`[data-testid="export-markdown"]`)
	if err != nil {
		t.Fatal(err)
	}
	href, err := exportLink.Attribute("href")
	if err != nil || href == nil {
		t.Fatal("export href missing")
	}
	result, err := page.Eval(`async (url) => {
		const r = await fetch(url, { credentials: 'same-origin' });
		const text = await r.text();
		return { status: r.status, text: text };
	}`, *href)
	if err != nil {
		t.Fatalf("export fetch: %v", err)
	}
	if int(result.Value.Get("status").Num()) != 200 {
		t.Fatalf("export status=%v", result.Value.Get("status"))
	}
	md := result.Value.Get("text").Str()
	if !strings.Contains(md, surface) {
		t.Fatalf("export missing surface %q body=%q", surface, md)
	}
}

func TestUI_PhotoIngest_OCRFail_ErrorVisible_QueueUnchanged(t *testing.T) {
	// Fail via test double; queue stays empty. (Real engine fail covered in L1/L2 with MustEngine.)
	base, shutdown := startServerWith(t, ocr.Static{Err: fmt.Errorf("ocr boom")})
	t.Cleanup(shutdown)
	imgPath := fixtureImagePath(t, "19_not_an_image")

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	input, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-input"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := input.SetFiles([]string{imgPath}); err != nil {
		t.Fatalf("set photo file: %v", err)
	}
	submit, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-submit"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := submit.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}

	el, err := page.Timeout(15 * time.Second).Element(`[data-testid="page-text-error"]`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("OCR error missing: %v\nhtml=%s", err, html)
	}
	txt, _ := el.Text()
	if txt == "" {
		t.Fatal("expected OCR error text")
	}
	// Phone-readable: OCR failure + retake or paste fallback.
	lower := strings.ToLower(txt)
	if !strings.Contains(lower, "could not read") && !strings.Contains(lower, "no text found") {
		t.Fatalf("OCR error not clear: %q", txt)
	}
	if !strings.Contains(lower, "paste") && !strings.Contains(lower, "retake") && !strings.Contains(lower, "photo") {
		t.Fatalf("OCR error missing next step: %q", txt)
	}

	// Queue empty state unchanged
	nav, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := nav.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-empty"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue should stay empty after OCR fail: %v\nhtml=%s", err, html)
	}
}

// TestUI_ShipGate_PhotoUpload_Export_ClearAll is the ticket-08 end-to-end ship gate:
// PIN → upload fixture → pick → analyze → mark → export (queue remains) → Clear all → empty.
func TestUI_ShipGate_PhotoUpload_Export_ClearAll(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	imgPath := fixtureImagePath(t, "02_multi_sentence")

	browser := newBrowser(t)
	page := unlockToShell(t, browser, base)

	input, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-input"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := input.SetFiles([]string{imgPath}); err != nil {
		t.Fatalf("set photo file: %v", err)
	}
	submit, err := page.Timeout(5 * time.Second).Element(`[data-testid="photo-submit"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := submit.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(15 * time.Second).Element(`[data-testid="sentence-candidates"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("candidates: %v\nhtml=%s", err, html)
	}

	pick, err := page.Timeout(5 * time.Second).Element(`[data-testid="candidate-pick"][data-index="0"]`)
	if err != nil {
		picks, err2 := page.Elements(`[data-testid="candidate-pick"]`)
		if err2 != nil || len(picks) == 0 {
			html, _ := page.HTML()
			t.Fatalf("candidate-pick missing: %v\nhtml=%s", err, html)
		}
		pick = picks[0]
	}
	if err := pick.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("analyze: %v\nhtml=%s", err, html)
	}

	words, err := page.Elements(`[data-testid="mark-unknown"]`)
	if err != nil || len(words) == 0 {
		html, _ := page.HTML()
		t.Fatalf("no mark-unknown\nhtml=%s", html)
	}
	surfaceAttr, err := words[0].Attribute("data-surface")
	if err != nil || surfaceAttr == nil || *surfaceAttr == "" {
		t.Fatal("mark-unknown missing data-surface")
	}
	surface := *surfaceAttr
	if err := words[0].Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	fb, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("save feedback: %v\nhtml=%s", err, html)
	}
	if txt, _ := fb.Text(); !strings.Contains(txt, surface) {
		t.Fatalf("feedback=%q surface=%q", txt, surface)
	}

	// Duplicate second tap — feedback must stay obvious.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		el, err := page.Element(`[data-testid="entry-id"]`)
		if err == nil {
			v, err := el.Property("value")
			if err == nil && v.Str() != "" {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	clickMarkUnknown(t, page, surface)
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="duplicate"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("duplicate feedback: %v\nhtml=%s", err, html)
	}

	nav, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := nav.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-entry"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-entry: %v\nhtml=%s", err, html)
	}

	exportLink, err := page.Timeout(5 * time.Second).Element(`[data-testid="export-markdown"]`)
	if err != nil {
		t.Fatal(err)
	}
	href, err := exportLink.Attribute("href")
	if err != nil || href == nil || *href == "" {
		t.Fatalf("export href missing: %v %v", href, err)
	}
	result, err := page.Eval(`async (url) => {
		const r = await fetch(url, { credentials: 'same-origin' });
		const text = await r.text();
		return { status: r.status, type: r.headers.get('content-type') || '', text: text };
	}`, *href)
	if err != nil {
		t.Fatalf("export fetch: %v", err)
	}
	if int(result.Value.Get("status").Num()) != 200 {
		t.Fatalf("export status=%v", result.Value.Get("status"))
	}
	if !strings.Contains(result.Value.Get("type").Str(), "markdown") {
		t.Fatalf("content-type=%q", result.Value.Get("type").Str())
	}
	md := result.Value.Get("text").Str()
	if !strings.Contains(md, surface) {
		t.Fatalf("export missing surface %q body=%q", surface, md)
	}

	// Export must leave queue intact.
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="queue-entry"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue gone after export: %v\nhtml=%s", err, html)
	}

	waitDialog, handleDialog := page.MustHandleDialog()
	btn, err := page.Timeout(5 * time.Second).Element(`[data-testid="clear-all"]`)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = btn.Click(proto.InputMouseButtonLeft, 1)
	}()
	waitDialog()
	handleDialog(true, "")

	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-empty"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-empty after clear: %v\nhtml=%s", err, html)
	}
	entries, err := page.Elements(`[data-testid="queue-entry"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries after clear=%d", len(entries))
	}
	clearBtn, err := page.Timeout(5 * time.Second).Element(`[data-testid="clear-all"]`)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := clearBtn.Property("disabled")
	if err != nil {
		t.Fatal(err)
	}
	if !disabled.Bool() {
		t.Fatal("clear-all should be disabled when empty")
	}
}
