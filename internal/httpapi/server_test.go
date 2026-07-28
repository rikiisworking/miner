package httpapi_test

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func newTestServer(t *testing.T) *httpapi.Server {
	t.Helper()
	return newTestServerWith(t, analyzer.Stub{}, queuestore.NewMem(), ocr.Stub{})
}

func newTestServerWith(t *testing.T, a ports.JapaneseAnalyzer, q ports.QueueStore, o ports.OcrEngine) *httpapi.Server {
	t.Helper()
	if q == nil {
		q = queuestore.NewMem()
	}
	if o == nil {
		o = ocr.Stub{}
	}
	m := app.NewMiningApp(pinauth.Static{Secret: "test-pin-ok"}, a, q, o)
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

// Restart (new process) uses a fresh in-memory session store — old cookie must not unlock.
func TestSession_InvalidatedOnServerRestart(t *testing.T) {
	s1 := newTestServer(t)
	cookies := unlockCookies(t, s1)

	// Pre-restart: gated route OK
	reqOK := httptest.NewRequest(http.MethodGet, "/home", nil)
	for _, c := range cookies {
		reqOK.AddCookie(c)
	}
	respOK, err := s1.App().Test(reqOK, -1)
	if err != nil {
		t.Fatal(err)
	}
	respOK.Body.Close()
	if respOK.StatusCode != http.StatusOK {
		t.Fatalf("pre-restart /home status=%d", respOK.StatusCode)
	}

	// New server instance = process restart (new memory session store)
	s2 := newTestServer(t)
	req2 := httptest.NewRequest(http.MethodGet, "/home", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	req2.Header.Set("Accept", "application/json")
	resp2, err := s2.App().Test(req2, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("post-restart /home status=%d want 401 body=%s", resp2.StatusCode, body)
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

func TestPageText_Authenticated_ReturnsCandidates(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)

	form := url.Values{"page_text": {"病院に行った。今日は雨だ。私は本を読む。"}}
	req := httptest.NewRequest(http.MethodPost, "/page-text", strings.NewReader(form.Encode()))
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
	if !strings.Contains(html, `data-testid="sentence-candidates"`) {
		t.Fatalf("missing candidates: %s", html)
	}
	if strings.Count(html, `data-testid="sentence-candidate"`) != 3 {
		t.Fatalf("want 3 candidates: %s", html)
	}
	if !strings.Contains(html, "病院に行った。") || !strings.Contains(html, "私は本を読む。") {
		t.Fatalf("missing sentence text: %s", html)
	}
	if !strings.Contains(html, `hx-post="/analyze"`) {
		t.Fatalf("candidates should post to analyze: %s", html)
	}
}

func TestPageText_Empty_ClearError(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)

	form := url.Values{"page_text": {"   "}}
	req := httptest.NewRequest(http.MethodPost, "/page-text", strings.NewReader(form.Encode()))
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
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `data-testid="page-text-error"`) {
		t.Fatalf("missing page-text-error: %s", body)
	}
}

func TestPageText_Unauthenticated_Rejected(t *testing.T) {
	s := newTestServer(t)
	form := url.Values{"page_text": {"病院に行った。"}}
	req := httptest.NewRequest(http.MethodPost, "/page-text", strings.NewReader(form.Encode()))
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

func TestPageText_ThenAnalyze_SelectCandidateOverHTTP(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)

	// Propose
	form := url.Values{"page_text": {"病院に行った。私は本を読む。"}}
	req := httptest.NewRequest(http.MethodPost, "/page-text", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("page-text status=%d", resp.StatusCode)
	}

	// Select second candidate via analyze (same as pick button)
	form2 := url.Values{"sentence": {"私は本を読む。"}}
	req2 := httptest.NewRequest(http.MethodPost, "/analyze", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	resp2, err := s.App().Test(req2, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body, _ := io.ReadAll(resp2.Body)
	html := string(body)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("analyze status=%d body=%s", resp2.StatusCode, html)
	}
	if !strings.Contains(html, `data-testid="analyze-success"`) {
		t.Fatalf("missing analyze-success: %s", html)
	}
	if !strings.Contains(html, `data-surface="本"`) {
		t.Fatalf("expected content for selected sentence: %s", html)
	}
	// OOB working-sentence textarea updated
	if !strings.Contains(html, `hx-swap-oob="true"`) || !strings.Contains(html, "私は本を読む。") {
		t.Fatalf("expected OOB sentence sync: %s", html)
	}
}

func TestHome_ShowsPageTextSection(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
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
	if !strings.Contains(html, `data-testid="page-text-section"`) {
		t.Fatalf("missing page-text-section: %s", html)
	}
	if !strings.Contains(html, `data-testid="page-text-input"`) {
		t.Fatalf("missing page-text-input: %s", html)
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

func TestAddUnknown_Authenticated_QueueListReflectsEntry(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)

	form := url.Values{
		"sentence": {"私は本を読む。"},
		"surface":  {"本"},
		"entry_id": {""},
	}
	req := httptest.NewRequest(http.MethodPost, "/unknowns", strings.NewReader(form.Encode()))
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
	if !strings.Contains(html, `data-status="saved"`) {
		t.Fatalf("expected saved feedback: %s", html)
	}
	if !strings.Contains(html, "本") {
		t.Fatalf("expected surface in feedback: %s", html)
	}
	// OOB entry_id present for subsequent taps
	if !strings.Contains(html, `data-testid="entry-id"`) || !strings.Contains(html, `value="`) {
		t.Fatalf("expected entry_id in response: %s", html)
	}

	qreq := httptest.NewRequest(http.MethodGet, "/queue", nil)
	for _, c := range cookies {
		qreq.AddCookie(c)
	}
	qresp, err := s.App().Test(qreq, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer qresp.Body.Close()
	qbody, _ := io.ReadAll(qresp.Body)
	qhtml := string(qbody)
	if qresp.StatusCode != http.StatusOK {
		t.Fatalf("queue status=%d body=%s", qresp.StatusCode, qhtml)
	}
	if !strings.Contains(qhtml, `data-testid="queue-entry"`) {
		t.Fatalf("missing queue entry: %s", qhtml)
	}
	if !strings.Contains(qhtml, "私は本を読む。") {
		t.Fatalf("missing sentence: %s", qhtml)
	}
	if !strings.Contains(qhtml, `data-testid="queue-unknown"`) || !strings.Contains(qhtml, `data-surface="本"`) {
		t.Fatalf("missing unknown: %s", qhtml)
	}
}

// Same pass_id with empty entry_id must bind both posts to one queue entry (transport proof of Pass protocol).
func TestAddUnknown_SamePassID_EmptyEntryID_OneEntry(t *testing.T) {
	q := queuestore.NewMem()
	s := newTestServerWith(t, analyzer.Stub{}, q, ocr.Stub{})
	cookies := unlockCookies(t, s)

	post := func(surface, passID string) string {
		t.Helper()
		// Omit empty entry_id: pass_id alone drives create-or-bind (Pass protocol).
		body := "sentence=" + url.QueryEscape("病院に行った。") +
			"&surface=" + url.QueryEscape(surface) +
			"&pass_id=" + url.QueryEscape(passID)
		req := httptest.NewRequest(http.MethodPost, "/unknowns", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := s.App().Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.StatusCode, raw)
		}
		return string(raw)
	}

	const pass = "pass-l2-shared"
	html1 := post("病院", pass)
	if !strings.Contains(html1, `data-status="saved"`) {
		t.Fatalf("first save: %s", html1)
	}
	html2 := post("行った", pass)
	if !strings.Contains(html2, `data-status="saved"`) {
		t.Fatalf("second save (same pass, different surface): %s", html2)
	}

	list, err := q.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("entries=%d want 1 (same pass_id); unknowns=%v", len(list), list)
	}
	if len(list[0].Unknowns) != 2 || list[0].Unknowns[0] != "病院" || list[0].Unknowns[1] != "行った" {
		t.Fatalf("unknowns=%v want [病院 行った]", list[0].Unknowns)
	}
}

func TestAddUnknown_Duplicate_IdempotentOneUnknown(t *testing.T) {
	q := queuestore.NewMem()
	s := newTestServerWith(t, analyzer.Stub{}, q, ocr.Stub{})
	cookies := unlockCookies(t, s)

	post := func(entryID string) string {
		t.Helper()
		form := url.Values{
			"sentence": {"病院に行った。"},
			"surface":  {"病院"},
			"entry_id": {entryID},
		}
		req := httptest.NewRequest(http.MethodPost, "/unknowns", strings.NewReader(form.Encode()))
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
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.StatusCode, body)
		}
		return string(body)
	}

	html1 := post("")
	if !strings.Contains(html1, `data-status="saved"`) {
		t.Fatalf("first save: %s", html1)
	}
	// Extract entry id from hidden input value=
	entryID := extractAttr(html1, `id="entry_id"`, "value")
	if entryID == "" {
		// try data-testid form
		entryID = extractHiddenValue(html1, "entry_id")
	}
	if entryID == "" {
		t.Fatalf("could not parse entry_id from: %s", html1)
	}

	html2 := post(entryID)
	if !strings.Contains(html2, `data-status="duplicate"`) {
		t.Fatalf("dup feedback: %s", html2)
	}

	list, err := q.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("entries=%d want 1", len(list))
	}
	if len(list[0].Unknowns) != 1 || list[0].Unknowns[0] != "病院" {
		t.Fatalf("unknowns=%v want [病院]", list[0].Unknowns)
	}
}

func TestQueue_Unauthenticated_Rejected(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/queue", nil)
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

func extractHiddenValue(html, name string) string {
	// crude: name="entry_id" ... value="..."
	marker := `name="` + name + `"`
	i := strings.Index(html, marker)
	if i < 0 {
		marker = `id="` + name + `"`
		i = strings.Index(html, marker)
	}
	if i < 0 {
		return ""
	}
	rest := html[i:]
	v := strings.Index(rest, `value="`)
	if v < 0 {
		return ""
	}
	rest = rest[v+len(`value="`):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func extractAttr(html, near, attr string) string {
	i := strings.Index(html, near)
	if i < 0 {
		return ""
	}
	// search a window after near
	window := html[i:]
	if len(window) > 200 {
		window = window[:200]
	}
	key := attr + `="`
	j := strings.Index(window, key)
	if j < 0 {
		return ""
	}
	rest := window[j+len(key):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func TestExport_Authenticated_MarkdownUTF8_QueueUnchanged(t *testing.T) {
	q := queuestore.NewMem()
	// Seed store directly — L2 asserts transport, not AddUnknown form round-trip.
	t0 := mustParseTime(t, "2026-01-01T00:00:00Z")
	if err := q.Create(ports.QueueEntry{
		ID: "e1", Sentence: "病院に行った。", Unknowns: []string{"病院", "行った"}, FirstUnknownAt: t0,
	}); err != nil {
		t.Fatal(err)
	}
	if err := q.Create(ports.QueueEntry{
		ID: "e2", Sentence: "今日は雨だ。", Unknowns: []string{"雨"}, FirstUnknownAt: t0.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	s := newTestServerWith(t, analyzer.Stub{}, q, ocr.Stub{})
	cookies := unlockCookies(t, s)

	before, err := q.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 {
		t.Fatalf("setup entries=%d", len(before))
	}

	ereq := httptest.NewRequest(http.MethodGet, "/export", nil)
	for _, c := range cookies {
		ereq.AddCookie(c)
	}
	eresp, err := s.App().Test(ereq, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer eresp.Body.Close()
	body, _ := io.ReadAll(eresp.Body)
	if eresp.StatusCode != http.StatusOK {
		t.Fatalf("export status=%d body=%s", eresp.StatusCode, body)
	}
	ct := eresp.Header.Get("Content-Type")
	if !strings.Contains(ct, "markdown") {
		t.Fatalf("Content-Type=%q want markdown", ct)
	}
	cd := eresp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, "miner-export.md") {
		t.Fatalf("Content-Disposition=%q", cd)
	}
	want := "- 病院に行った。\n  - 病院\n  - 行った\n- 今日は雨だ。\n  - 雨\n"
	if string(body) != want {
		t.Fatalf("export body:\n%s\nwant:\n%s", body, want)
	}

	after, err := q.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("export mutated queue: before=%d after=%d", len(before), len(after))
	}
}

func mustParseTime(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestExport_EmptyQueue_OKEmptyBody(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)
	req := httptest.NewRequest(http.MethodGet, "/export", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, body)
	}
	if len(body) != 0 {
		t.Fatalf("want empty body, got %q", body)
	}
}

func TestExport_Unauthenticated_Rejected(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/export", nil)
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

func TestClearAll_EmptiesQueue_SecondClearSafe(t *testing.T) {
	q := queuestore.NewMem()
	s := newTestServerWith(t, analyzer.Stub{}, q, ocr.Stub{})
	cookies := unlockCookies(t, s)

	form := url.Values{"sentence": {"今日は雨だ。"}, "surface": {"雨"}}
	req := httptest.NewRequest(http.MethodPost, "/unknowns", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	clear := func() *http.Response {
		t.Helper()
		creq := httptest.NewRequest(http.MethodPost, "/queue/clear", nil)
		for _, c := range cookies {
			creq.AddCookie(c)
		}
		cresp, err := s.App().Test(creq, -1)
		if err != nil {
			t.Fatal(err)
		}
		return cresp
	}

	cresp := clear()
	// Fiber Test may not follow redirects; accept 303 or 200
	if cresp.StatusCode != http.StatusSeeOther && cresp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cresp.Body)
		cresp.Body.Close()
		t.Fatalf("clear status=%d body=%s", cresp.StatusCode, body)
	}
	cresp.Body.Close()

	list, err := q.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("after clear entries=%d", len(list))
	}

	cresp2 := clear()
	if cresp2.StatusCode != http.StatusSeeOther && cresp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cresp2.Body)
		cresp2.Body.Close()
		t.Fatalf("second clear status=%d body=%s", cresp2.StatusCode, body)
	}
	cresp2.Body.Close()
}

func TestQueuePage_ShowsExportAndClearControls(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)
	req := httptest.NewRequest(http.MethodGet, "/queue", nil)
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
	if !strings.Contains(html, `data-testid="export-markdown"`) {
		t.Fatalf("missing export control: %s", html)
	}
	if !strings.Contains(html, `data-testid="clear-all"`) {
		t.Fatalf("missing clear-all control: %s", html)
	}
	if !strings.Contains(html, `disabled`) {
		t.Fatalf("empty queue should disable clear-all: %s", html)
	}
}

// Ensure file-backed store works end-to-end through HTTP (optional smoke).
func TestAddUnknown_FileStore_Persists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	s := newTestServerWith(t, analyzer.Stub{}, queuestore.NewFile(path), ocr.Stub{})
	cookies := unlockCookies(t, s)

	form := url.Values{
		"sentence": {"今日は雨だ。"},
		"surface":  {"雨"},
	}
	req := httptest.NewRequest(http.MethodPost, "/unknowns", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Reopen store
	list, err := queuestore.NewFile(path).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || len(list[0].Unknowns) != 1 || list[0].Unknowns[0] != "雨" {
		t.Fatalf("persisted=%+v", list)
	}
}

// multipartIngest builds a POST /ingest body with one file field "image".
func multipartIngest(t *testing.T, filename string, image []byte) (*http.Request, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("image", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(image); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/ingest", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, w.FormDataContentType()
}

func TestIngest_Authenticated_TinyFixture_ReturnsCandidates(t *testing.T) {
	manifest, err := ocrtest.Load()
	if err != nil {
		t.Fatal(err)
	}
	c := manifest.Must("02_multi_sentence")
	img, err := c.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	engine := ocr.Stub{ByBytes: map[string]string{
		string(img): "病院に行った。今日は雨だ。私は本を読む。",
	}}
	s := newTestServerWith(t, analyzer.Stub{}, queuestore.NewMem(), engine)
	cookies := unlockCookies(t, s)

	req, _ := multipartIngest(t, "page.png", img)
	for _, ck := range cookies {
		req.AddCookie(ck)
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
	if !strings.Contains(html, `data-testid="sentence-candidates"`) {
		t.Fatalf("missing candidates: %s", html)
	}
	if strings.Count(html, `data-testid="sentence-candidate"`) != 3 {
		t.Fatalf("want 3 candidates: %s", html)
	}
	if !strings.Contains(html, "病院に行った。") || !strings.Contains(html, "私は本を読む。") {
		t.Fatalf("missing sentence text: %s", html)
	}
}

func TestIngest_Unauthenticated_Rejected(t *testing.T) {
	req, _ := multipartIngest(t, "page.png", []byte("fake-png"))
	req.Header.Set("Accept", "application/json")
	s := newTestServer(t)

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", resp.StatusCode)
	}
}

func TestIngest_Oversize_ClearError(t *testing.T) {
	// Under Fiber BodyLimit (Max+512KiB) so MiningApp rejects, not the framework.
	img := make([]byte, app.MaxUploadBytes+1)
	for i := range img {
		img[i] = byte(i % 251)
	}
	s := newTestServerWith(t, analyzer.Stub{}, queuestore.NewMem(), ocr.Stub{Text: "should-not-run"})
	cookies := unlockCookies(t, s)

	req, _ := multipartIngest(t, "big.png", img)
	for _, ck := range cookies {
		req.AddCookie(ck)
	}

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d want 413 body=%s", resp.StatusCode, html)
	}
	if !strings.Contains(html, `data-testid="page-text-error"`) && !strings.Contains(html, `data-testid="candidates-error"`) {
		t.Fatalf("missing error partial: %s", html)
	}
	if !strings.Contains(html, "10") || !strings.Contains(strings.ToLower(html), "large") {
		t.Fatalf("error should mention size cap: %s", html)
	}
}

func TestIngest_OCRFailure_QueueIntact(t *testing.T) {
	q := queuestore.NewMem()
	// Seed one durable entry before failing OCR.
	if err := q.Create(ports.QueueEntry{
		ID:             "seed-1",
		Sentence:       "病院に行った。",
		Unknowns:       []string{"病院"},
		FirstUnknownAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	s := newTestServerWith(t, analyzer.Stub{}, q, ocr.Stub{FailWith: errors.New("engine down")})
	cookies := unlockCookies(t, s)

	req, _ := multipartIngest(t, "page.png", []byte("tiny-img"))
	for _, ck := range cookies {
		req.AddCookie(ck)
	}

	resp, err := s.App().Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422 body=%s", resp.StatusCode, html)
	}
	if !strings.Contains(html, `role="alert"`) {
		t.Fatalf("missing alert: %s", html)
	}
	if !strings.Contains(strings.ToLower(html), "image") && !strings.Contains(strings.ToLower(html), "ocr") && !strings.Contains(strings.ToLower(html), "photo") {
		t.Fatalf("OCR fail message unclear: %s", html)
	}

	// Queue still has the seed entry.
	list, err := q.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "seed-1" {
		t.Fatalf("queue corrupted after OCR fail: %+v", list)
	}

	// GET /queue still lists it.
	reqQ := httptest.NewRequest(http.MethodGet, "/queue", nil)
	for _, ck := range cookies {
		reqQ.AddCookie(ck)
	}
	respQ, err := s.App().Test(reqQ, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer respQ.Body.Close()
	qBody, _ := io.ReadAll(respQ.Body)
	if !strings.Contains(string(qBody), "病院に行った。") {
		t.Fatalf("queue page lost entry: %s", qBody)
	}
}

func TestHome_ShowsPhotoUploadSection(t *testing.T) {
	s := newTestServer(t)
	cookies := unlockCookies(t, s)
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
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
	if !strings.Contains(html, `data-testid="photo-upload-section"`) {
		t.Fatalf("missing photo-upload-section: %s", html)
	}
	if !strings.Contains(html, `data-testid="photo-input"`) {
		t.Fatalf("missing photo-input: %s", html)
	}
	if !strings.Contains(html, `hx-post="/ingest"`) {
		t.Fatalf("photo form should post /ingest: %s", html)
	}
	if !strings.Contains(html, `hx-encoding="multipart/form-data"`) {
		t.Fatalf("photo form needs multipart encoding: %s", html)
	}
}
