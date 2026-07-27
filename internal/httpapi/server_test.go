package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/httpapi"
	"github.com/rikiisworking/miner/internal/ports"
	"github.com/rikiisworking/miner/web"
)

type fakePinAuth struct {
	valid string
}

func (f fakePinAuth) Verify(pin string) bool { return pin == f.valid }

func newTestServer(t *testing.T) *httpapi.Server {
	t.Helper()
	return newTestServerWith(t, analyzer.Stub{})
}

func newTestServerWith(t *testing.T, a ports.JapaneseAnalyzer) *httpapi.Server {
	t.Helper()
	m := app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"}, a)
	s, err := httpapi.New(httpapi.Config{
		MiningApp: m,
		WebFS:     web.FS(),
		Addr:      ":0",
	})
	if err != nil {
		t.Fatalf("New server: %v", err)
	}
	return s
}

func unlockCookies(t *testing.T, s *httpapi.Server) []*http.Cookie {
	t.Helper()
	form := url.Values{"pin": {"test-pin-ok"}}
	req := httptest.NewRequest(http.MethodPost, "/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unlock status=%d body=%s", resp.StatusCode, body)
	}
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		// Fiber may only expose Set-Cookie header
		for _, sc := range resp.Header.Values("Set-Cookie") {
			part := strings.SplitN(sc, ";", 2)[0]
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				cookies = append(cookies, &http.Cookie{Name: strings.TrimSpace(kv[0]), Value: kv[1]})
			}
		}
	}
	if len(cookies) == 0 {
		t.Fatal("no session cookie after unlock")
	}
	return cookies
}

func TestUnlock_WrongPIN_Unauthorized(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{"pin": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401; body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "Incorrect PIN") {
		t.Fatalf("body missing error: %s", body)
	}
	if strings.Contains(string(body), `data-testid="app-shell"`) {
		t.Fatal("mining shell must not appear on wrong PIN")
	}
}

func TestUnlock_CorrectPIN_SetsSessionCookieAndShell(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{"pin": {"test-pin-ok"}}
	req := httptest.NewRequest(http.MethodPost, "/unlock", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `data-testid="app-shell"`) {
		t.Fatalf("expected app shell: %s", body)
	}
	if !strings.Contains(string(body), `data-testid="sentence-input"`) {
		t.Fatalf("expected analyze form on shell: %s", body)
	}

	setCookie := resp.Header.Values("Set-Cookie")
	if len(setCookie) == 0 {
		t.Fatal("expected Set-Cookie for session")
	}
	joined := strings.Join(setCookie, "\n")
	lower := strings.ToLower(joined)
	if !strings.Contains(lower, "httponly") {
		t.Fatalf("cookie missing HttpOnly: %s", joined)
	}
	if !strings.Contains(lower, "samesite=lax") {
		t.Fatalf("cookie missing SameSite=Lax: %s", joined)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/home", nil)
	for _, c := range resp.Cookies() {
		req2.AddCookie(c)
	}
	if len(resp.Cookies()) == 0 {
		for _, sc := range setCookie {
			part := strings.SplitN(sc, ";", 2)[0]
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				req2.AddCookie(&http.Cookie{Name: strings.TrimSpace(kv[0]), Value: kv[1]})
			}
		}
	}
	resp2, err := s.App().Test(req2, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("/home status=%d body=%s", resp2.StatusCode, body2)
	}
	if !strings.Contains(string(body2), `data-testid="app-shell"`) {
		t.Fatalf("/home missing shell: %s", body2)
	}
}

func TestHome_WithoutSession_Rejected(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	req.Header.Set("Accept", "application/json")

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}

func TestAnalyze_Authenticated_FixtureSentence_RubyAndContent(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)

	form := url.Values{"sentence": {"私は本を読む。"}}
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, html)
	}
	if !strings.Contains(html, `data-testid="analyze-success"`) {
		t.Fatalf("missing analyze-success: %s", html)
	}
	if !strings.Contains(html, "<ruby") || !strings.Contains(html, "<rt>") {
		t.Fatalf("expected HTML ruby furigana: %s", html)
	}
	if !strings.Contains(html, "わたし") || !strings.Contains(html, "ほん") {
		t.Fatalf("expected readings in body: %s", html)
	}
	if !strings.Contains(html, `data-testid="content-word"`) {
		t.Fatalf("expected content-word rows: %s", html)
	}
	// Particle は must not appear as a content-word surface alone.
	if strings.Contains(html, `data-surface="は"`) {
		t.Fatalf("particle must not be content-word: %s", html)
	}
	if strings.Contains(html, `data-surface="を"`) {
		t.Fatalf("particle must not be content-word: %s", html)
	}
	if !strings.Contains(html, `data-surface="私"`) || !strings.Contains(html, `data-surface="本"`) {
		t.Fatalf("expected content surfaces: %s", html)
	}
}

func TestAnalyze_Unauthenticated_Rejected(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{"sentence": {"私は本を読む。"}}
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}

func TestAnalyze_Unauthenticated_HTMX_AuthErrorFragment(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{"sentence": {"私は本を読む。"}}
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401; body=%s", resp.StatusCode, html)
	}
	if !strings.Contains(html, `data-testid="auth-error"`) {
		t.Fatalf("expected auth-error fragment: %s", html)
	}
	if strings.Contains(html, `data-testid="analyze-error"`) {
		t.Fatal("session gate must not render analyze-error")
	}
	if strings.Contains(html, `data-testid="analyze-success"`) {
		t.Fatal("session gate must not render analyze-success")
	}
	if strings.Contains(html, `data-testid="content-word"`) {
		t.Fatal("session gate must not render analyze content")
	}
}

func TestAnalyze_Failure_ClearError(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)

	form := url.Values{"sentence": {analyzer.ForceErrorText}}
	req := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422; body=%s", resp.StatusCode, html)
	}
	if !strings.Contains(html, `data-testid="analyze-error"`) {
		t.Fatalf("missing analyze-error: %s", html)
	}
	if !strings.Contains(html, "could not be tokenized") {
		t.Fatalf("expected clear error message: %s", html)
	}
	if strings.Contains(html, `data-testid="analyze-success"`) {
		t.Fatal("success fragment must not appear on analyze failure")
	}
}
