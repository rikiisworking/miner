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
	"github.com/rikiisworking/miner/internal/adapters/pinauth"
	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/httpapi"
	"github.com/rikiisworking/miner/web"
)

func startServer(t *testing.T) (baseURL string, shutdown func()) {
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
	if txt, _ := el.Text(); txt == "" {
		t.Fatal("expected analyze error text")
	}
	has, _, _ := page.Has(`[data-testid="analyze-success"]`)
	if has {
		t.Fatal("analyze-success must not show on forced error")
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


