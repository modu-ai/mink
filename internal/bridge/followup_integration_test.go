// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-007, REQ-BR-009, REQ-BR-010, REQ-BR-014, REQ-BR-018
// AC:  AC-BR-007, AC-BR-009, AC-BR-010, AC-BR-016
// M5 follow-up — transport wire-in scenarios:
//
//   1. resumer.Resume hands buffered messages to a freshly registered
//      sender on WebSocket upgrade.
//   2. resumer.Resume hands buffered messages to a freshly registered
//      sender on SSE reconnect (Last-Event-ID equivalent).
//   3. AuthRequest exposes CookieHash on every failure path that has a
//      stable cookie identity, so RecordFailure is reachable from the
//      WS / SSE handlers (CodeRabbit Finding #4 wire-in).
//   4. dispatcher.SendOutbound no longer brackets the gate — the bracket
//      lives in the transport sender so sequential captureSender writes
//      (which do not call ObserveWrite) stay at zero stalls.
//   5. The sender bracket itself absorbs an ObserveWrite + ObserveDrain
//      pair per emit, so single-sender concurrent emits remain
//      double-count-free (REQ-BR-010).

package bridge

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/jonboulle/clockwork"
)

// followupBufferedSender returns the resumer-driven slice of OutboundMessages
// that the resumer hands back, plus a gate-aware sender that mimics
// wsSender / sseSender bracket semantics. The sender records each emit
// in the embedded captureSender so callers can assert on order + count.
func followupBufferedSender(t *testing.T, sessionID string, count int) (*resumer, *gateBracketSender, *flushGate) {
	t.Helper()
	clock := clockwork.NewFakeClock()
	buf := newOutboundBuffer(clock)
	for i := 1; i <= count; i++ {
		buf.Append(OutboundMessage{
			SessionID: sessionID,
			Type:      OutboundChunk,
			Payload:   fmt.Appendf(nil, `"msg-%d"`, i),
			Sequence:  uint64(i),
		})
	}
	gate := newFlushGate()
	return newResumer(buf), &gateBracketSender{gate: gate, sessionID: sessionID}, gate
}

// TestFollowupScenario1_WSResumerDrivesSenderBracket — when X-Last-Sequence
// names a buffered prefix, resumer.Resume returns the suffix and the
// transport sender emits each message in order. With the gate-aware sender
// modelling wsSender, ObserveWrite/Drain pairs cancel out and the gate
// returns to idle.
func TestFollowupScenario1_WSResumerDrivesSenderBracket(t *testing.T) {
	t.Parallel()

	res, sender, gate := followupBufferedSender(t, "cx", 5)

	headers := http.Header{}
	headers.Set(HeaderLastSequence, "2") // client saw seq 1+2; resume from 3.
	replay := res.Resume("cx", headers)

	if len(replay) != 3 {
		t.Fatalf("replay count = %d, want 3 (seq 3..5)", len(replay))
	}
	for i, msg := range replay {
		want := uint64(i + 3)
		if msg.Sequence != want {
			t.Fatalf("replay[%d] sequence = %d, want %d", i, msg.Sequence, want)
		}
		if err := sender.SendOutbound(msg); err != nil {
			t.Fatalf("sender err on seq %d: %v", msg.Sequence, err)
		}
	}

	got := sender.snapshot()
	if len(got) != 3 {
		t.Fatalf("sender captured %d, want 3", len(got))
	}
	if gate.Stalled("cx") {
		t.Fatalf("gate still stalled after sequential drain — bracket lost ObserveDrain")
	}
}

// TestFollowupScenario2_SSEResumerDrivesSenderBracket — Last-Event-ID
// drives the same resume path. The header constant differs (HeaderLastEventID)
// but parseLastSequence falls back to it when X-Last-Sequence is missing.
func TestFollowupScenario2_SSEResumerDrivesSenderBracket(t *testing.T) {
	t.Parallel()

	res, sender, gate := followupBufferedSender(t, "cx", 4)

	headers := http.Header{}
	headers.Set(HeaderLastEventID, "1") // browser saw event id=1; resume from 2.
	replay := res.Resume("cx", headers)

	if len(replay) != 3 {
		t.Fatalf("replay count = %d, want 3 (seq 2..4)", len(replay))
	}
	for _, msg := range replay {
		if err := sender.SendOutbound(msg); err != nil {
			t.Fatalf("sender err: %v", err)
		}
	}

	got := sender.snapshot()
	if len(got) != 3 || got[0].Sequence != 2 || got[2].Sequence != 4 {
		t.Fatalf("captured = %+v, want seqs 2,3,4", got)
	}
	if gate.Stalled("cx") {
		t.Fatalf("gate still stalled after SSE replay drain")
	}
}

// TestFollowupScenario3_AuthErrorCarriesCookieHash — every AuthRequest
// rejection that has a syntactically present cookie must surface its
// hash so the rate-limiter can count the failed attempt. Without this,
// CookieHash stays nil and the consecutive-failure streak never grows
// past zero.
func TestFollowupScenario3_AuthErrorCarriesCookieHash(t *testing.T) {
	t.Parallel()

	auth, err := NewAuthenticator(AuthConfig{HMACSecret: bytes.Repeat([]byte("k"), 32)})
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	revocation := NewRevocationStore(nil)

	cases := []struct {
		name      string
		mutate    func(req *http.Request)
		wantHash  bool
		wantError string
	}{
		{
			name: "no cookie",
			mutate: func(req *http.Request) {
				// no-op: cookie absent
			},
			wantHash:  false,
			wantError: "unauthenticated",
		},
		{
			name: "tampered cookie",
			mutate: func(req *http.Request) {
				req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "garbage.value"})
			},
			wantHash:  true,
			wantError: "unauthenticated",
		},
		{
			name: "valid cookie but bad origin",
			mutate: func(req *http.Request) {
				cookie, _, ierr := auth.IssueSessionCookie()
				if ierr != nil {
					t.Fatalf("issue: %v", ierr)
				}
				req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: cookie})
				req.Host = "evil.example.com" // not loopback
			},
			wantHash:  true,
			wantError: "bad_origin",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://localhost/bridge/ws", nil)
			req.Host = "localhost:1234"
			tc.mutate(req)

			_, _, authErr := AuthRequest(req, auth, revocation, false)
			if authErr == nil {
				t.Fatalf("expected AuthError, got nil")
			}
			if authErr.Reason != tc.wantError {
				t.Fatalf("Reason = %q, want %q", authErr.Reason, tc.wantError)
			}
			gotHash := len(authErr.CookieHash) > 0
			if gotHash != tc.wantHash {
				t.Fatalf("hash present = %v, want %v (rate-limit tracking depends on it)",
					gotHash, tc.wantHash)
			}
		})
	}
}

// TestFollowupScenario4_DispatcherDoesNotBracketGate — verify the
// bracket has actually been removed from dispatcher.SendOutbound. With
// the bracket relocated to wsSender/sseSender, a captureSender (no gate
// hookup) means a 100-frame burst no longer trips the stall counter.
func TestFollowupScenario4_DispatcherDoesNotBracketGate(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	gate := newFlushGate()
	disp := newOutboundDispatcher(reg, nil, gate, nil)

	cs := &captureSender{}
	if err := reg.Add(WebUISession{ID: "sx", Transport: TransportWebSocket, State: SessionStateActive}); err != nil {
		t.Fatalf("registry add: %v", err)
	}
	reg.RegisterSender("sx", cs)

	// 100 × 8 KiB = 800 KiB — would have tripped HighWatermarkBytes (256 KiB)
	// if the dispatcher were still calling ObserveWrite per emit.
	payload := make([]byte, 8*1024)
	for range 100 {
		if _, err := disp.SendOutbound("sx", OutboundChunk, payload); err != nil {
			t.Fatalf("send err: %v", err)
		}
	}

	if got := gate.Stalls(); got != 0 {
		t.Fatalf("gate stalls = %d, want 0 — dispatcher bracket regression", got)
	}
	if got := len(cs.snapshot()); got != 100 {
		t.Fatalf("captured = %d, want 100", got)
	}
}

// TestFollowupScenario5_SenderBracketBalancedAcrossSequentialWrites —
// when the bracket lives in the sender, sequential SendOutbound calls
// produce balanced ObserveWrite/ObserveDrain pairs and the gate stays
// idle even when payloads exceed the high watermark in aggregate.
func TestFollowupScenario5_SenderBracketBalancedAcrossSequentialWrites(t *testing.T) {
	t.Parallel()

	gate := newFlushGate()
	cs := &gateBracketSender{gate: gate, sessionID: "sx"}

	// Sequential emits: each ObserveWrite is followed immediately by the
	// deferred ObserveDrain inside SendOutbound, so in-flight bytes never
	// accumulate past one frame.
	payload := make([]byte, 8*1024)
	for i := uint64(1); i <= 100; i++ {
		if err := cs.SendOutbound(OutboundMessage{
			SessionID: "sx",
			Type:      OutboundChunk,
			Payload:   payload,
			Sequence:  i,
		}); err != nil {
			t.Fatalf("send %d err: %v", i, err)
		}
	}

	if got := gate.Stalls(); got != 0 {
		t.Fatalf("gate stalls = %d, want 0 — sender bracket leaked ObserveDrain", got)
	}
	if gate.Stalled("sx") {
		t.Fatal("gate still stalled at end of sequential bursts")
	}
}

// followupTestServer is a thin httptest wrapper that exercises BuildMux
// end-to-end so the auth + rate-limit + sender wire-in covers the actual
// HTTP path. Used by the BuildMux integration test below.
func followupTestServer(t *testing.T) (*httptest.Server, *Authenticator, *rateLimiter) {
	t.Helper()
	auth, err := NewAuthenticator(AuthConfig{HMACSecret: bytes.Repeat([]byte("k"), 32)})
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	reg := NewRegistry()
	revocation := NewRevocationStore(nil)
	rateLim := newRateLimiter(clockwork.NewFakeClock())
	mux := BuildMux(MuxConfig{
		Auth:        auth,
		Registry:    reg,
		Revocation:  revocation,
		RateLimiter: rateLim,
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, auth, rateLim
}

// TestFollowupHTTPLoop_AuthFailureIncrementsRateLimitStreak verifies the
// end-to-end wire-up: an HTTP-level WebSocket upgrade with a tampered
// cookie reaches AuthRequest, triggers RecordFailure inside ws.go, and
// the streak is observable via rateLimiter.Check on subsequent calls.
func TestFollowupHTTPLoop_AuthFailureIncrementsRateLimitStreak(t *testing.T) {
	t.Parallel()

	srv, auth, rateLim := followupTestServer(t)

	// Hammer the upgrade endpoint with a tampered cookie. Each request
	// is rejected with 401 + RecordFailure tally.
	tampered := "garbage." + strconv.Itoa(0)
	for i := 0; i < MaxConsecutiveFailures; i++ {
		req, _ := http.NewRequest("GET", srv.URL+"/bridge/ws", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: tampered})
		req.Header.Set("Origin", srv.URL)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("upgrade %d err: %v", i, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", i, resp.StatusCode)
		}
	}

	// After 10 failures the cookie hash should have crossed the streak
	// threshold; the rate-limiter must surface RateRequireFreshCookie.
	hash := auth.CookieHash(tampered)
	if got := rateLim.Check(hash); got != RateRequireFreshCookie {
		t.Fatalf("rate-limit Check = %v, want RateRequireFreshCookie after %d failures",
			got, MaxConsecutiveFailures)
	}
}
