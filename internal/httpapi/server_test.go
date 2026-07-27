package httpapi_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rikiisworking/miner/internal/adapters/analyzer"
	"github.com/rikiisworking/miner/internal/adapters/queuestore"
	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/httpapi"
	"github.com/rikiisworking/miner/internal/ports"
	"github.com/rikiisworking/miner/web"
)

type fakePinAuth struct {
	valid string
}

func (f fakePinAuth) Verify(pin string) bool { return pin == f.valid }

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

func newTestServer(t *testing.T) *httpapi.Server {
	t.Helper()
	return newTestServerWith(t, analyzer.Stub{}, newMemQueue())
}

func newTestServerWith(t *testing.T, a ports.JapaneseAnalyzer, q ports.QueueStore) *httpapi.Server {
	t.Helper()
	if q == nil {
		q = newMemQueue()
	}
	m := app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"}, a, q)
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

func TestAddUnknown_Duplicate_IdempotentOneUnknown(t *testing.T) {
	q := newMemQueue()
	s := newTestServerWith(t, analyzer.Stub{}, q)
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

// Ensure file-backed store works end-to-end through HTTP (optional smoke).
func TestAddUnknown_FileStore_Persists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	s := newTestServerWith(t, analyzer.Stub{}, queuestore.NewFile(path))
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
