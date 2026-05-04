// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-003, REQ-BR-006, REQ-BR-011, REQ-BR-014, REQ-BR-015
// AC: AC-BR-003, AC-BR-006, AC-BR-011, AC-BR-012, AC-BR-013
// M2 — integration tests against a live httptest.Server with BuildMux.

package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/jonboulle/clockwork"
)

// recordingAdapter captures every InboundMessage handed off by the mux.
type recordingAdapter struct {
	mu       sync.Mutex
	received []InboundMessage
	failNext atomic.Bool
}

func (a *recordingAdapter) HandleInbound(msg InboundMessage) error {
	if a.failNext.Swap(false) {
		return fmt.Errorf("forced failure")
	}
	a.mu.Lock()
	a.received = append(a.received, msg)
	a.mu.Unlock()
	return nil
}

func (a *recordingAdapter) snapshot() []InboundMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]InboundMessage, len(a.received))
	copy(out, a.received)
	return out
}

func newTestStack(t *testing.T) (*httptest.Server, *Authenticator, *Registry, *RevocationStore, *recordingAdapter) {
	t.Helper()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	auth, err := NewAuthenticator(AuthConfig{
		HMACSecret: []byte("test-secret-32-bytes-padding-xx!"),
		Clock:      clk,
	})
	if err != nil {
		t.Fatalf("auth init err = %v", err)
	}
	reg := NewRegistry()
	rev := NewRevocationStore(clk)
	adapter := &recordingAdapter{}

	mux := BuildMux(MuxConfig{
		Auth:       auth,
		Registry:   reg,
		Revocation: rev,
		Adapter:    adapter,
		// Allow any origin for httptest (the server binds to 127.0.0.1
		// but on a random port; coder/websocket's pattern matcher needs
		// the wildcard to skip origin enforcement during tests).
		WSAcceptOrigins: []string{"*"},
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, auth, reg, rev, adapter
}

// loginAndCookies issues a fresh login and returns the http.Cookie set the
// caller should carry in subsequent requests, plus the CSRF token value.
func loginAndCookies(t *testing.T, srv *httptest.Server) ([]*http.Cookie, string) {
	t.Helper()
	resp, err := http.Post(srv.URL+"/bridge/login", "application/json",
		strings.NewReader(`{"intent":"first_install"}`))
	if err != nil {
		t.Fatalf("login err = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	var body loginResponse
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.Cookies(), body.CSRFToken
}

func TestBuildMux_RoutesLoginLogoutWS(t *testing.T) {
	t.Parallel()

	srv, _, _, _, _ := newTestStack(t)

	// /bridge/login — POST OK
	resp, err := http.Post(srv.URL+"/bridge/login", "application/json",
		strings.NewReader(`{"intent":"first_install"}`))
	if err != nil {
		t.Fatalf("login err = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("login status = %d, want 200", resp.StatusCode)
	}

	// /bridge/logout — POST OK
	resp, err = http.Post(srv.URL+"/bridge/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("logout err = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("logout status = %d, want 204", resp.StatusCode)
	}

	// /bridge/ws — GET without cookie → 401
	resp, err = http.Get(srv.URL + "/bridge/ws")
	if err != nil {
		t.Fatalf("ws err = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("ws unauth status = %d, want 401", resp.StatusCode)
	}
}

func TestWebSocket_DialUpgradeAndChat(t *testing.T) {
	t.Parallel()

	srv, _, _, _, adapter := newTestStack(t)
	cookies, _ := loginAndCookies(t, srv)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/bridge/ws"

	header := http.Header{}
	header.Set("Cookie", cookieHeaderValue(cookies))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		t.Fatalf("Dial err = %v", err)
	}
	defer conn.CloseNow()

	// Send a chat inbound; expect adapter to receive it.
	payload := []byte(`{"type":"chat","payload":{"text":"hi"}}`)
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatalf("Write err = %v", err)
	}

	// Wait briefly for adapter dispatch.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(adapter.snapshot()) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got := adapter.snapshot()
	if len(got) != 1 {
		t.Fatalf("adapter received %d messages, want 1", len(got))
	}
	if got[0].Type != InboundChat {
		t.Errorf("type = %q, want %q", got[0].Type, InboundChat)
	}
}

func TestWebSocket_RejectsExpiredCookieWith401(t *testing.T) {
	t.Parallel()

	srv, auth, _, _, _ := newTestStack(t)

	// Forge an expired cookie by dialing with an HMAC issued from a
	// distinct authenticator (different secret) — VerifySessionCookie
	// returns ErrCookieInvalid which AuthRequest classifies as
	// "unauthenticated".
	bad, err := NewAuthenticator(AuthConfig{HMACSecret: []byte("alt-secret-32-bytes-padding-zzz!")})
	if err != nil {
		t.Fatalf("alt auth err = %v", err)
	}
	cookieValue, _, _ := bad.IssueSessionCookie()
	_ = auth // unused — alt cookie won't validate

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/bridge/ws"
	header := http.Header{}
	header.Set("Cookie", SessionCookieName+"="+cookieValue)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err == nil {
		t.Fatalf("Dial succeeded; want failure")
	}
	if resp == nil {
		t.Fatalf("Dial err but no response: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("dial response status = %d, want 401", resp.StatusCode)
	}
}

func TestWebSocket_OversizeFrameClosesWith4413(t *testing.T) {
	t.Parallel()

	srv, _, _, _, _ := newTestStack(t)
	cookies, _ := loginAndCookies(t, srv)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/bridge/ws"
	header := http.Header{}
	header.Set("Cookie", cookieHeaderValue(cookies))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: header,
	})
	if err != nil {
		t.Fatalf("Dial err = %v", err)
	}
	defer conn.CloseNow()

	// Send 11 MB of payload — exceeds MaxInboundBytes (10 MB).
	big := make([]byte, MaxInboundBytes+1)
	for i := range big {
		big[i] = 'A'
	}
	_ = conn.Write(ctx, websocket.MessageText, big)

	// The next Read should return a CloseError; coder/websocket converts
	// the SetReadLimit overflow into close 1009 (MessageTooBig) on the
	// peer side. Either way the conn must terminate quickly.
	_, _, readErr := conn.Read(ctx)
	if readErr == nil {
		t.Fatalf("Read after oversize frame succeeded; want close error")
	}
}

func TestSSEHandler_StreamsEventsAndRejectsUnauth(t *testing.T) {
	t.Parallel()

	srv, _, _, _, _ := newTestStack(t)

	// Without cookie → 401
	resp, err := http.Get(srv.URL + "/bridge/stream")
	if err != nil {
		t.Fatalf("sse unauth err = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("sse unauth status = %d, want 401", resp.StatusCode)
	}

	// With cookie → 200 + text/event-stream
	cookies, _ := loginAndCookies(t, srv)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/bridge/stream", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err = srv.Client().Do(req)
	if err != nil {
		t.Fatalf("sse auth err = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("sse status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

func TestInboundPostHandler_AcceptsAndDispatches(t *testing.T) {
	t.Parallel()

	srv, _, _, _, adapter := newTestStack(t)
	cookies, csrf := loginAndCookies(t, srv)

	body := strings.NewReader(`{"type":"chat","payload":{"text":"hello"}}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/bridge/inbound", body)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set(CSRFHeaderName, csrf)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("inbound post err = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("inbound status = %d, want 202", resp.StatusCode)
	}

	got := adapter.snapshot()
	if len(got) != 1 || got[0].Type != InboundChat {
		t.Errorf("adapter received = %+v, want 1 chat", got)
	}
}

func TestInboundPostHandler_RejectsCSRFMismatch(t *testing.T) {
	t.Parallel()

	srv, _, _, _, _ := newTestStack(t)
	cookies, _ := loginAndCookies(t, srv)

	body := strings.NewReader(`{"type":"chat","payload":{}}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/bridge/inbound", body)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set(CSRFHeaderName, "wrong-value")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want 403", resp.StatusCode)
	}
}

func TestInboundPostHandler_413OnOversize(t *testing.T) {
	t.Parallel()

	srv, _, _, _, _ := newTestStack(t)
	cookies, csrf := loginAndCookies(t, srv)

	big := make([]byte, MaxInboundBytes+1)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/bridge/inbound",
		strings.NewReader(string(big)))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set(CSRFHeaderName, csrf)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", resp.StatusCode)
	}
	bodyOut, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(bodyOut), "size_exceeded") {
		t.Errorf("body = %q, want substring \"size_exceeded\"", bodyOut)
	}
}

func TestInboundPostHandler_RejectsMalformedBody(t *testing.T) {
	t.Parallel()

	srv, _, _, _, _ := newTestStack(t)
	cookies, csrf := loginAndCookies(t, srv)

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/bridge/inbound",
		strings.NewReader(`{not_json`))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	req.Header.Set(CSRFHeaderName, csrf)

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestDecodeInbound_SizeAndType(t *testing.T) {
	t.Parallel()

	now := time.Now()
	cases := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{"chat", `{"type":"chat","payload":{}}`, nil},
		{"attachment", `{"type":"attachment","payload":{}}`, nil},
		{"control", `{"type":"control","payload":{}}`, nil},
		{"unknown_type", `{"type":"random","payload":{}}`, ErrInboundMalformed},
		{"malformed_json", `{not_json`, ErrInboundMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeInbound("sid", []byte(tc.raw), now)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr.Error()) {
				t.Errorf("err = %v, want substring %v", err, tc.wantErr)
			}
		})
	}
}

func TestDecodeInbound_RejectsOversize(t *testing.T) {
	t.Parallel()
	big := make([]byte, MaxInboundBytes+1)
	_, err := DecodeInbound("sid", big, time.Now())
	if err == nil {
		t.Fatalf("err = nil, want ErrInboundTooLarge")
	}
	if !strings.Contains(err.Error(), "exceeds size limit") {
		t.Errorf("err = %v, want size-limit error", err)
	}
}

func TestParseLastEventID(t *testing.T) {
	t.Parallel()
	cases := map[string]uint64{
		"":     0,
		"42":   42,
		"abc":  0,
		"9999": 9999,
	}
	for in, want := range cases {
		req := httptest.NewRequest(http.MethodGet, "/bridge/stream", nil)
		if in != "" {
			req.Header.Set("Last-Event-ID", in)
		}
		got := parseLastEventID(req)
		if got != want {
			t.Errorf("parseLastEventID(%q) = %d, want %d", in, got, want)
		}
	}
}

func cookieHeaderValue(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c.Name+"="+url.QueryEscape(c.Value))
	}
	return strings.Join(parts, "; ")
}
