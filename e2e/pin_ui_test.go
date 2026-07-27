package e2e_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"

	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/httpapi"
	"github.com/rikiisworking/miner/web"
)

type fakePinAuth struct {
	valid string
}

func (f fakePinAuth) Verify(pin string) bool { return pin == f.valid }

func startServer(t *testing.T) (baseURL string, shutdown func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	m := app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"})
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
}
