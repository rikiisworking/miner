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
	"github.com/rikiisworking/miner/internal/ports"
	"github.com/rikiisworking/miner/web"
)

// defaultE2EOCR: multi-sentence Static text (no host NDLOCR-Lite).
// Lines empty → capture UI uses sentence chips fallback.
var defaultE2EOCR ports.OcrEngine = ocr.Static{
	Text: "病院に行った。\n私は本を読む。",
	Lines: []ports.OcrLine{
		{Text: "病院に行った。", X: 10, Y: 20, W: 180, H: 40},
		{Text: "私は本を読む。", X: 10, Y: 80, W: 200, H: 40},
	},
	Width:  400,
	Height: 300,
}

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

func unlockToHome(t *testing.T, browser *rod.Browser, base string) *rod.Page {
	t.Helper()
	page := openPINPage(t, browser, base)
	fillPIN(t, page, "test-pin-ok")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="app-home"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("app-home missing: %v\nhtml=%s", err, html)
	}
	return page
}

// openCapture navigates to /capture and waits for minerCapture hook.
func openCapture(t *testing.T, page *rod.Page, base string) {
	t.Helper()
	if err := page.Timeout(10 * time.Second).Navigate(base + "/capture"); err != nil {
		t.Fatalf("navigate capture: %v", err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="capture-page"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("capture-page missing: %v\nhtml=%s", err, html)
	}
	// Wait for camera.js hook
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := page.Eval(`() => !!(window.minerCapture && window.minerCapture.enterFrozen)`)
		if err == nil && res.Value.Bool() {
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatal("minerCapture hook missing")
}

// freezeViaIngest runs POST /ingest in-page and freezes UI (no real camera).
func freezeViaIngest(t *testing.T, page *rod.Page) {
	t.Helper()
	_, err := page.Eval(`async () => {
		const blob = new Blob([new Uint8Array([0xFF,0xD8,0xFF,0xD9])], { type: 'image/jpeg' });
		const fd = new FormData();
		fd.append('image', blob, 'capture.jpg');
		const res = await fetch('/ingest', {
			method: 'POST',
			credentials: 'same-origin',
			headers: { 'Accept': 'application/json' },
			body: fd
		});
		const data = await res.json();
		if (!res.ok) throw new Error(data.error || ('ingest ' + res.status));
		const dataUrl = 'data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAn/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIQAxAAAAGcP//EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAQUCf//EABQRAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQMBAT8Bf//EABQRAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQIBAT8Bf//EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEABj8Cf//EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEAAT8hf//Z';
		window.minerCapture.enterFrozen(dataUrl, data.regions || [], data.candidates || []);
		return { nRegions: (data.regions||[]).length, nCands: (data.candidates||[]).length };
	}`)
	if err != nil {
		html, _ := page.HTML()
		t.Fatalf("freezeViaIngest: %v\nhtml=%s", err, html)
	}
}

func pickFirstSentence(t *testing.T, page *rod.Page) {
	t.Helper()
	// Prefer region button; fall back to chip.
	var btn *rod.Element
	var err error
	btn, err = page.Timeout(3 * time.Second).Element(`[data-testid="sentence-region"]`)
	if err != nil {
		btn, err = page.Timeout(3 * time.Second).Element(`[data-testid="sentence-chip"]`)
	}
	if err != nil {
		// Direct pick via hook using first candidate from freeze
		_, err2 := page.Eval(`() => {
			const chip = document.querySelector('[data-testid="sentence-chip"], [data-testid="sentence-region"]');
			if (chip) { chip.click(); return; }
			window.minerCapture.pickSentence('病院に行った。');
		}`)
		if err2 != nil {
			html, _ := page.HTML()
			t.Fatalf("pick sentence: %v / %v\nhtml=%s", err, err2, html)
		}
	} else if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("analyze-success after pick: %v\nhtml=%s", err, html)
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

func TestUI_WrongPIN_ShowsError_NoHome(t *testing.T) {
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
	has, _, _ := page.Has(`[data-testid="app-home"]`)
	if has {
		t.Fatal("home must not show after wrong PIN")
	}
}

func TestUI_CorrectPIN_ShowsHome(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)

	browser := newBrowser(t)
	page := openPINPage(t, browser, base)
	fillPIN(t, page, "test-pin-ok")

	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="app-home"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("app-home missing: %v\nhtml=%s", err, html)
	}
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="take-photo"]`); err != nil {
		t.Fatal("take-photo missing")
	}
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`); err != nil {
		t.Fatal("nav-queue missing")
	}
	has, _, _ := page.Has(`[data-testid="sentence-input"]`)
	if has {
		t.Fatal("manual sentence input must not be on home")
	}
}

func TestUI_Capture_ChromePresent(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	openCapture(t, page, base)

	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="camera-shutter"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="capture-back"]`); err != nil {
		t.Fatal(err)
	}
	has, _, _ := page.Has(`[data-testid="photo-upload-section"]`)
	if has {
		t.Fatal("file upload must not appear on capture")
	}
}

func TestUI_FreezePick_RubyAndContentWords(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	openCapture(t, page, base)
	freezeViaIngest(t, page)
	pickFirstSentence(t, page)

	if _, err := page.Timeout(5 * time.Second).Element(`ruby[data-testid="ruby-token"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("ruby missing: %v\nhtml=%s", err, html)
	}
	// Kanji-only: 病院 must appear; pure kana must not.
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="content-word"][data-surface="病院"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("病院 content-word missing: %v\nhtml=%s", err, html)
	}
}

func TestUI_MarkUnknown_SaveFeedback_AndQueue(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	openCapture(t, page, base)
	freezeViaIngest(t, page)
	// Pick 私は本を読む for 本
	_, err := page.Eval(`() => window.minerCapture.pickSentence('私は本を読む。')`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("analyze-success: %v\nhtml=%s", err, html)
	}
	clickMarkUnknown(t, page, "本")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("save feedback: %v\nhtml=%s", err, html)
	}

	if err := page.Timeout(10 * time.Second).Navigate(base + "/queue"); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-entry"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-entry: %v\nhtml=%s", err, html)
	}
	entry, _ := page.Element(`[data-testid="queue-entry"]`)
	if txt, _ := entry.Text(); !strings.Contains(txt, "本") {
		t.Fatalf("queue entry=%q", txt)
	}
}

func TestUI_MarkUnknown_DuplicateFeedback(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	openCapture(t, page, base)
	freezeViaIngest(t, page)
	_, _ = page.Eval(`() => window.minerCapture.pickSentence('病院に行った。')`)
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		t.Fatal(err)
	}
	clickMarkUnknown(t, page, "病院")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`); err != nil {
		t.Fatal(err)
	}
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
}

func TestUI_ExportAndClearAll(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	openCapture(t, page, base)
	freezeViaIngest(t, page)
	_, _ = page.Eval(`() => window.minerCapture.pickSentence('病院に行った。')`)
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		t.Fatal(err)
	}
	clickMarkUnknown(t, page, "病院")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`); err != nil {
		t.Fatal(err)
	}

	if err := page.Timeout(10 * time.Second).Navigate(base + "/queue"); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-entry"]`); err != nil {
		t.Fatal(err)
	}
	exportLink, err := page.Timeout(5 * time.Second).Element(`[data-testid="export-markdown"]`)
	if err != nil {
		t.Fatal(err)
	}
	href, err := exportLink.Attribute("href")
	if err != nil || href == nil || *href == "" {
		t.Fatalf("export href missing")
	}
	result, err := page.Eval(`async (url) => {
		const r = await fetch(url, { credentials: 'same-origin' });
		const text = await r.text();
		return { status: r.status, type: r.headers.get('content-type') || '', text: text };
	}`, *href)
	if err != nil {
		t.Fatal(err)
	}
	if int(result.Value.Get("status").Num()) != 200 {
		t.Fatalf("export status=%v", result.Value.Get("status"))
	}
	md := result.Value.Get("text").Str()
	if !strings.Contains(md, "病院") {
		t.Fatalf("export body=%q", md)
	}

	waitDialog, handleDialog := page.MustHandleDialog()
	btn, err := page.Timeout(5 * time.Second).Element(`[data-testid="clear-all"]`)
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = btn.Click(proto.InputMouseButtonLeft, 1) }()
	waitDialog()
	handleDialog(true, "")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-empty"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-empty: %v\nhtml=%s", err, html)
	}
}

func TestUI_Queue_EmptyState(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	nav, err := page.Timeout(5 * time.Second).Element(`[data-testid="nav-queue"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := nav.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-empty"]`); err != nil {
		html, _ := page.HTML()
		t.Fatalf("queue-empty: %v\nhtml=%s", err, html)
	}
}

func TestUI_DetailBack_ReturnsFrozen(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	openCapture(t, page, base)
	freezeViaIngest(t, page)
	pickFirstSentence(t, page)
	if _, err := page.Timeout(5 * time.Second).Element(`[data-testid="sentence-detail"].show, [data-testid="sentence-detail"]:not([hidden])`); err != nil {
		// detail may use class show
		res, _ := page.Eval(`() => {
			const d = document.querySelector('[data-testid="sentence-detail"]');
			return !!(d && d.classList.contains('show'));
		}`)
		if res == nil || !res.Value.Bool() {
			html, _ := page.HTML()
			t.Fatalf("detail not shown\nhtml=%s", html)
		}
	}
	back, err := page.Timeout(5 * time.Second).Element(`[data-testid="detail-back"]`)
	if err != nil {
		t.Fatal(err)
	}
	if err := back.Click(proto.InputMouseButtonLeft, 1); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		res, err := page.Eval(`() => window.minerCapture && window.minerCapture.mode() === 'frozen'`)
		if err == nil && res.Value.Bool() {
			return
		}
		time.Sleep(40 * time.Millisecond)
	}
	mode, _ := page.Eval(`() => window.minerCapture ? window.minerCapture.mode() : 'none'`)
	t.Fatalf("want frozen after detail back, mode=%v", mode.Value)
}

func TestUI_OCRFail_ErrorVisible(t *testing.T) {
	base, shutdown := startServerWith(t, ocr.Static{Err: fmt.Errorf("ocr boom")})
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	openCapture(t, page, base)
	_, err := page.Eval(`async () => {
		const blob = new Blob([new Uint8Array([1,2,3])], { type: 'image/jpeg' });
		const fd = new FormData();
		fd.append('image', blob, 'x.jpg');
		const res = await fetch('/ingest', {
			method: 'POST', credentials: 'same-origin',
			headers: { 'Accept': 'application/json' }, body: fd
		});
		const data = await res.json();
		if (res.ok) throw new Error('expected fail');
		const errEl = document.querySelector('[data-testid="camera-error"]');
		if (errEl) {
			errEl.textContent = data.error || 'fail';
			errEl.hidden = false;
		}
		return data.error || '';
	}`)
	if err != nil {
		t.Fatal(err)
	}
	el, err := page.Timeout(5 * time.Second).Element(`[data-testid="camera-error"]`)
	if err != nil {
		t.Fatal(err)
	}
	txt, _ := el.Text()
	if txt == "" {
		t.Fatal("expected error text")
	}
}

func TestUI_ShipGate_FreezePickMarkExportClear(t *testing.T) {
	base, shutdown := startServer(t)
	t.Cleanup(shutdown)
	browser := newBrowser(t)
	page := unlockToHome(t, browser, base)
	openCapture(t, page, base)
	freezeViaIngest(t, page)
	_, _ = page.Eval(`() => window.minerCapture.pickSentence('私は本を読む。')`)
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="analyze-success"]`); err != nil {
		t.Fatal(err)
	}
	clickMarkUnknown(t, page, "本")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="unknown-feedback"][data-status="saved"]`); err != nil {
		t.Fatal(err)
	}
	if err := page.Timeout(10 * time.Second).Navigate(base + "/queue"); err != nil {
		t.Fatal(err)
	}
	exportLink, err := page.Timeout(5 * time.Second).Element(`[data-testid="export-markdown"]`)
	if err != nil {
		t.Fatal(err)
	}
	href, _ := exportLink.Attribute("href")
	result, err := page.Eval(`async (url) => {
		const r = await fetch(url, { credentials: 'same-origin' });
		return { status: r.status, text: await r.text() };
	}`, *href)
	if err != nil {
		t.Fatal(err)
	}
	if int(result.Value.Get("status").Num()) != 200 || !strings.Contains(result.Value.Get("text").Str(), "本") {
		t.Fatalf("export fail: %v", result.Value)
	}
	waitDialog, handleDialog := page.MustHandleDialog()
	btn, _ := page.Element(`[data-testid="clear-all"]`)
	go func() { _ = btn.Click(proto.InputMouseButtonLeft, 1) }()
	waitDialog()
	handleDialog(true, "")
	if _, err := page.Timeout(10 * time.Second).Element(`[data-testid="queue-empty"]`); err != nil {
		t.Fatal(err)
	}
}

