package httpapi_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rikiisworking/miner/internal/app"
	"github.com/rikiisworking/miner/internal/httpapi"
	"github.com/rikiisworking/miner/web"
)

type fakePinAuth struct {
	valid string
}

func (f fakePinAuth) Verify(pin string) bool { return pin == f.valid }

func newTestServer(t *testing.T) *httpapi.Server {
	t.Helper()
	m := app.NewMiningApp(fakePinAuth{valid: "test-pin-ok"})
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
