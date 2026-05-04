// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-002, REQ-BR-014, REQ-BR-016 (v0.2.0)
// AC: AC-BR-002, AC-BR-012, AC-BR-014
// M1-T3, M1-T4, M1-T5 — login/logout HTTP handlers + AuthRequest helper.

package bridge

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func newTestLoginStack(t *testing.T) (*Authenticator, *Registry, *RevocationStore, clockwork.FakeClock) {
	t.Helper()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	auth, err := NewAuthenticator(AuthConfig{
		HMACSecret: []byte("test-secret-32-bytes-padding-xx!"),
		Clock:      clk,
	})
	if err != nil {
		t.Fatalf("NewAuthenticator err = %v", err)
	}
	return auth, NewRegistry(), NewRevocationStore(clk), clk
}

func TestLoginHandler_AcceptsFirstInstall(t *testing.T) {
	t.Parallel()

	auth, _, _, clk := newTestLoginStack(t)
	h := NewLoginHandler(auth)

	rr := httptest.NewRecorder()
	body := strings.NewReader(`{"intent":"first_install"}`)
	req := httptest.NewRequest(http.MethodPost, "/bridge/login", body)
	req.Host = "127.0.0.1:8091"
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}

	cookies := rr.Result().Cookies()
	var sessCookie, csrfCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case SessionCookieName:
			sessCookie = c
		case CSRFCookieName:
			csrfCookie = c
		}
	}
	if sessCookie == nil {
		t.Fatalf("Set-Cookie %s missing", SessionCookieName)
	}
	if !sessCookie.HttpOnly {
		t.Errorf("session cookie HttpOnly = false, want true")
	}
	if sessCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("session cookie SameSite = %v, want Strict", sessCookie.SameSite)
	}
	if sessCookie.Path != "/" {
		t.Errorf("session cookie Path = %q, want /", sessCookie.Path)
	}
	if sessCookie.MaxAge != int(cookieLifetime.Seconds()) {
		t.Errorf("session cookie Max-Age = %d, want %d", sessCookie.MaxAge, int(cookieLifetime.Seconds()))
	}
	if csrfCookie == nil {
		t.Fatalf("Set-Cookie %s missing", CSRFCookieName)
	}
	if csrfCookie.HttpOnly {
		t.Errorf("csrf cookie HttpOnly = true, want false (double-submit)")
	}

	var resp loginResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response decode err = %v; body=%s", err, rr.Body.String())
	}
	if resp.CSRFToken == "" || resp.CSRFToken != csrfCookie.Value {
		t.Errorf("response csrf %q != cookie csrf %q", resp.CSRFToken, csrfCookie.Value)
	}
	if resp.ExpiresAt == "" {
		t.Errorf("expires_at empty")
	}

	// Verify the issued cookie validates and lands within the 24h window.
	sid, exp, err := auth.VerifySessionCookie(sessCookie.Value)
	if err != nil {
		t.Fatalf("issued cookie failed verification: %v", err)
	}
	if sid == "" {
		t.Errorf("verified session id empty")
	}
	wantExp := clk.Now().Add(cookieLifetime)
	if !exp.Equal(wantExp) {
		t.Errorf("expires = %v, want %v", exp, wantExp)
	}
}

func TestLoginHandler_AcceptsResume(t *testing.T) {
	t.Parallel()
	auth, _, _, _ := newTestLoginStack(t)
	h := NewLoginHandler(auth)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bridge/login",
		strings.NewReader(`{"intent":"resume"}`))
	req.Host = "127.0.0.1:8091"
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestLoginHandler_RejectsGET(t *testing.T) {
	t.Parallel()
	auth, _, _, _ := newTestLoginStack(t)
	h := NewLoginHandler(auth)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bridge/login", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
	if rr.Header().Get("Allow") != http.MethodPost {
		t.Errorf("Allow header = %q, want POST", rr.Header().Get("Allow"))
	}
}

func TestLoginHandler_RejectsUnknownIntent(t *testing.T) {
	t.Parallel()
	auth, _, _, _ := newTestLoginStack(t)
	h := NewLoginHandler(auth)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bridge/login",
		strings.NewReader(`{"intent":"unknown_action"}`))
	req.Host = "127.0.0.1:8091"
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

func TestLoginHandler_RejectsMalformedBody(t *testing.T) {
	t.Parallel()
	auth, _, _, _ := newTestLoginStack(t)
	h := NewLoginHandler(auth)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bridge/login",
		strings.NewReader(`{not json`))
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
}

// fakeCloser records every Close() invocation for assertion.
type fakeCloser struct {
	closed atomic.Int32
	last   atomic.Uint32 // last close code observed
	mu     sync.Mutex
	codes  []CloseCode
}

func (f *fakeCloser) Close(code CloseCode) error {
	f.closed.Add(1)
	f.last.Store(uint32(code))
	f.mu.Lock()
	f.codes = append(f.codes, code)
	f.mu.Unlock()
	return nil
}

func TestLogoutHandler_RevokesAndClosesAllSessions(t *testing.T) {
	t.Parallel()

	auth, reg, rev, _ := newTestLoginStack(t)
	logout := NewLogoutHandler(auth, reg, rev)

	cookieValue, _, _ := auth.IssueSessionCookie()
	hash := auth.CookieHash(cookieValue)

	// Register 3 sessions sharing the same cookie hash, each with a fakeCloser.
	closers := make([]*fakeCloser, 3)
	for i, id := range []string{"sess-a", "sess-b", "sess-c"} {
		_ = reg.Add(WebUISession{
			ID: id, CookieHash: hash, Transport: TransportWebSocket,
			OpenedAt: time.Now(), LastActivity: time.Now(), State: SessionStateActive,
		})
		closers[i] = &fakeCloser{}
		reg.RegisterCloser(id, closers[i])
	}

	// And one unrelated session that must NOT be closed.
	other, _, _ := auth.IssueSessionCookie()
	otherHash := auth.CookieHash(other)
	_ = reg.Add(WebUISession{
		ID: "sess-other", CookieHash: otherHash, Transport: TransportSSE,
		OpenedAt: time.Now(), LastActivity: time.Now(), State: SessionStateActive,
	})
	otherCloser := &fakeCloser{}
	reg.RegisterCloser("sess-other", otherCloser)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bridge/logout", nil)
	req.Host = "127.0.0.1:8091"
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookieValue})

	start := time.Now()
	logout.ServeHTTP(rr, req)
	elapsed := time.Since(start)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if elapsed > 2*time.Second {
		t.Errorf("logout took %v, want <=2s (REQ-BR-016)", elapsed)
	}
	if !rev.IsRevoked(hash) {
		t.Errorf("cookie hash not revoked after logout")
	}

	for i, c := range closers {
		if c.closed.Load() != 1 {
			t.Errorf("closer[%d] closed count = %d, want 1", i, c.closed.Load())
		}
		if CloseCode(c.last.Load()) != CloseSessionRevoked {
			t.Errorf("closer[%d] code = %d, want %d (4403)",
				i, c.last.Load(), CloseSessionRevoked)
		}
	}
	if otherCloser.closed.Load() != 0 {
		t.Errorf("unrelated closer was invoked: count = %d", otherCloser.closed.Load())
	}
}

func TestLogoutHandler_NoCookieIsIdempotent(t *testing.T) {
	t.Parallel()

	auth, reg, rev, _ := newTestLoginStack(t)
	logout := NewLogoutHandler(auth, reg, rev)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/bridge/logout", nil)
	req.Host = "127.0.0.1:8091"

	logout.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rr.Code)
	}
	if rev.Len() != 0 {
		t.Errorf("revocation store grew without cookie: %d", rev.Len())
	}
}

func TestLogoutHandler_RejectsGET(t *testing.T) {
	t.Parallel()
	auth, reg, rev, _ := newTestLoginStack(t)
	logout := NewLogoutHandler(auth, reg, rev)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/bridge/logout", nil)
	logout.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rr.Code)
	}
}

func TestAuthRequest_AcceptsValidCookieAndCSRF(t *testing.T) {
	t.Parallel()

	auth, _, rev, _ := newTestLoginStack(t)
	cookieValue, _, _ := auth.IssueSessionCookie()
	csrf, _ := auth.IssueCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/bridge/inbound", nil)
	req.Host = "127.0.0.1:8091"
	req.Header.Set("Origin", "http://127.0.0.1:8091")
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookieValue})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	req.Header.Set(CSRFHeaderName, csrf)

	sid, hash, err := AuthRequest(req, auth, rev, true)
	if err != nil {
		t.Fatalf("AuthRequest err = %v", err)
	}
	if sid == "" || len(hash) == 0 {
		t.Errorf("sid=%q hash-len=%d, want non-empty", sid, len(hash))
	}
}

func TestAuthRequest_RejectsMissingCookie(t *testing.T) {
	t.Parallel()
	auth, _, rev, _ := newTestLoginStack(t)
	req := httptest.NewRequest(http.MethodGet, "/bridge/ws", nil)
	req.Host = "127.0.0.1:8091"
	_, _, err := AuthRequest(req, auth, rev, false)
	if err == nil || err.Reason != "unauthenticated" {
		t.Errorf("err = %+v, want unauthenticated", err)
	}
}

func TestAuthRequest_RejectsExpired(t *testing.T) {
	t.Parallel()
	auth, _, rev, clk := newTestLoginStack(t)
	cookie, _, _ := auth.IssueSessionCookie()
	clk.Advance(25 * time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/bridge/ws", nil)
	req.Host = "127.0.0.1:8091"
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
	_, _, err := AuthRequest(req, auth, rev, false)
	if err == nil || err.Reason != "unauthenticated" {
		t.Errorf("err = %+v, want unauthenticated (expired)", err)
	}
}

func TestAuthRequest_RejectsRevoked(t *testing.T) {
	t.Parallel()
	auth, _, rev, _ := newTestLoginStack(t)
	cookie, _, _ := auth.IssueSessionCookie()
	rev.Revoke(auth.CookieHash(cookie))

	req := httptest.NewRequest(http.MethodGet, "/bridge/ws", nil)
	req.Host = "127.0.0.1:8091"
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
	_, _, err := AuthRequest(req, auth, rev, false)
	if err == nil || err.Reason != "revoked" {
		t.Errorf("err = %+v, want revoked", err)
	}
}

func TestAuthRequest_RejectsCSRFMismatch(t *testing.T) {
	t.Parallel()
	auth, _, rev, _ := newTestLoginStack(t)
	cookie, _, _ := auth.IssueSessionCookie()
	csrf, _ := auth.IssueCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/bridge/inbound", nil)
	req.Host = "127.0.0.1:8091"
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	req.Header.Set(CSRFHeaderName, csrf+"-tampered")

	_, _, err := AuthRequest(req, auth, rev, true)
	if err == nil || err.Reason != "csrf_mismatch" {
		t.Errorf("err = %+v, want csrf_mismatch", err)
	}
}

func TestAuthRequest_RejectsCSRFMissingHeader(t *testing.T) {
	t.Parallel()
	auth, _, rev, _ := newTestLoginStack(t)
	cookie, _, _ := auth.IssueSessionCookie()
	csrf, _ := auth.IssueCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/bridge/inbound", nil)
	req.Host = "127.0.0.1:8091"
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	// No X-CSRF-Token header.

	_, _, err := AuthRequest(req, auth, rev, true)
	if err == nil || err.Reason != "csrf_mismatch" {
		t.Errorf("err = %+v, want csrf_mismatch (missing header)", err)
	}
}

func TestAuthRequest_RejectsBadOrigin(t *testing.T) {
	t.Parallel()
	auth, _, rev, _ := newTestLoginStack(t)
	cookie, _, _ := auth.IssueSessionCookie()

	req := httptest.NewRequest(http.MethodGet, "/bridge/ws", nil)
	req.Host = "evil.example.com"
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})

	_, _, err := AuthRequest(req, auth, rev, false)
	if err == nil || err.Reason != "bad_origin" {
		t.Errorf("err = %+v, want bad_origin", err)
	}
}

func TestAuthRequest_RejectsCrossOriginEvenIfHostLoopback(t *testing.T) {
	t.Parallel()
	auth, _, rev, _ := newTestLoginStack(t)
	cookie, _, _ := auth.IssueSessionCookie()

	req := httptest.NewRequest(http.MethodGet, "/bridge/ws", nil)
	req.Host = "127.0.0.1:8091"
	req.Header.Set("Origin", "http://attacker.example.com")
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})

	_, _, err := AuthRequest(req, auth, rev, false)
	if err == nil || err.Reason != "bad_origin" {
		t.Errorf("err = %+v, want bad_origin (cross-origin)", err)
	}
}

func TestAuthError_Message(t *testing.T) {
	t.Parallel()
	err := &AuthError{Reason: "csrf_mismatch"}
	if got := err.Error(); !strings.Contains(got, "csrf_mismatch") {
		t.Errorf("AuthError.Error() = %q, want substring \"csrf_mismatch\"", got)
	}
}

func TestNewRevocationStore_NilClockDefaultsToReal(t *testing.T) {
	t.Parallel()
	store := NewRevocationStore(nil)
	if store == nil {
		t.Fatalf("NewRevocationStore(nil) returned nil")
	}
	store.Revoke([]byte("h"))
	if !store.IsRevoked([]byte("h")) {
		t.Errorf("IsRevoked = false, want true")
	}
}

func TestRevocationStore_RejectsEmptyHash(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	store := NewRevocationStore(clk)
	store.Revoke(nil)              // no-op
	store.Revoke([]byte{})         // no-op
	if store.IsRevoked(nil) {
		t.Errorf("IsRevoked(nil) = true, want false")
	}
	if store.Len() != 0 {
		t.Errorf("Len = %d, want 0", store.Len())
	}
}

func TestRevocationStore_ExpiresAfterLifetime(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	store := NewRevocationStore(clk)
	hash := []byte("hash-1")
	store.Revoke(hash)
	if !store.IsRevoked(hash) {
		t.Fatalf("IsRevoked just-after-revoke = false, want true")
	}
	clk.Advance(cookieLifetime + time.Second)
	if store.IsRevoked(hash) {
		t.Errorf("IsRevoked after lifetime = true, want false")
	}
}

func TestRevocationStore_GarbageCollectsOnRevoke(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	store := NewRevocationStore(clk)
	store.Revoke([]byte("h1"))
	clk.Advance(25 * time.Hour)
	store.Revoke([]byte("h2")) // triggers gc
	if store.Len() != 1 {
		t.Errorf("Len = %d, want 1 (h1 expired)", store.Len())
	}
}

func TestHostIsLoopback_TableDriven(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"127.0.0.1:8091": true,
		"localhost:8091": true,
		"[::1]:8091":     true,
		"127.0.0.1":      true,
		"::1":            false, // bare ::1 not in host:port form; treated as host "::" → false
		"evil.com:8091":  false,
		"10.0.0.1:8091":  false,
		"":               false,
	}
	for hostport, want := range cases {
		t.Run(hostport, func(t *testing.T) {
			t.Parallel()
			got := hostIsLoopback(hostport)
			if got != want {
				t.Errorf("hostIsLoopback(%q) = %v, want %v", hostport, got, want)
			}
		})
	}
}
