// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-004
// AC: AC-BR-AMEND-004, AC-BR-AMEND-005
// M4-T1 — cross-connection replay integration tests.
//
// These tests verify that a new connID sharing the same (CookieHash, Transport)
// as a prior connID can replay messages buffered under the prior connection.
// This is the core of the amendment: messages buffered under LogicalID L1 by
// connID sid-aaa are visible to connID sid-bbb when both share L1.
//
// Test naming convention:
//   TestCrossConnReplay_<Scenario> where Scenario maps to an AC.

package bridge

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/jonboulle/clockwork"
)

// newCrossConnHarness constructs the full dispatcher + buffer + resumer +
// registry + auth stack needed for cross-connection replay tests.
// All components mirror server.New wiring without the HTTP listener.
func newCrossConnHarness(t *testing.T) (
	auth *Authenticator,
	reg *Registry,
	buf *outboundBuffer,
	disp *outboundDispatcher,
	res *resumer,
) {
	t.Helper()
	var err error
	auth, err = NewAuthenticator(AuthConfig{HMACSecret: bytes.Repeat([]byte("k"), 32)})
	if err != nil {
		t.Fatalf("new authenticator: %v", err)
	}
	reg = NewRegistry()
	buf = newOutboundBuffer(clockwork.NewFakeClock())
	gate := newFlushGate()
	disp = newOutboundDispatcher(reg, buf, gate, nil)
	// Wire logout hook (mirrors server.New ordering).
	reg.SetLogoutHook(disp.dropLogicalBuffer)
	res = newResumer(buf, reg)
	return
}

// addSessionWithLogicalID registers a WebUISession with the LogicalID derived
// from (cookieHash, transport). The sender is registered when non-nil.
func addSessionWithLogicalID(
	t *testing.T,
	auth *Authenticator,
	reg *Registry,
	connID string,
	cookieHash []byte,
	transport Transport,
	sender SessionSender,
) {
	t.Helper()
	lid := auth.DeriveLogicalID(cookieHash, transport)
	if err := reg.Add(WebUISession{
		ID:         connID,
		CookieHash: cookieHash,
		Transport:  transport,
		LogicalID:  lid,
		State:      SessionStateActive,
	}); err != nil {
		t.Fatalf("registry add %s: %v", connID, err)
	}
	if sender != nil {
		reg.RegisterSender(connID, sender)
	}
}

// TestCrossConnReplay_FullReplayWebSocket — AC-BR-AMEND-004: full replay.
//
// Scenario:
//  1. sid-aaa connects (WS, cookie c1) → LogicalID = L1.
//  2. Dispatcher emits 5 chunks to sid-aaa (sequence 1-5). Buffer L1 holds 5.
//  3. sid-aaa disconnects: unregister sender + remove session. Buffer L1 intact.
//  4. sid-bbb connects (WS, same cookie c1) → same LogicalID = L1.
//  5. resumer.Resume("sid-bbb", X-Last-Sequence: 0) returns all 5 chunks.
func TestCrossConnReplay_FullReplayWebSocket(t *testing.T) {
	t.Parallel()

	auth, reg, buf, disp, res := newCrossConnHarness(t)

	// Fixed cookie hash: 32 zero bytes — deterministic across the test.
	cookieHash := make([]byte, 32)
	logicalID := auth.DeriveLogicalID(cookieHash, TransportWebSocket)

	// Step 1: connect sid-aaa.
	csA := &captureSender{}
	addSessionWithLogicalID(t, auth, reg, "sid-aaa", cookieHash, TransportWebSocket, csA)

	// Step 2: emit 5 chunks through dispatcher.
	for i := range 5 {
		seq, err := disp.SendOutbound("sid-aaa", OutboundChunk, []byte(`"payload"`))
		if err != nil {
			t.Fatalf("SendOutbound[%d]: %v", i, err)
		}
		if seq != uint64(i+1) {
			t.Fatalf("sequence[%d] = %d, want %d", i, seq, i+1)
		}
	}

	// sid-aaa should have received 5 live emits.
	if got := len(csA.snapshot()); got != 5 {
		t.Fatalf("sid-aaa live emits = %d, want 5", got)
	}

	// Verify buffer L1 holds 5 entries.
	if got := buf.Len(logicalID); got != 5 {
		t.Fatalf("buffer[%s] = %d, want 5", logicalID, got)
	}

	// Step 3: sid-aaa disconnects.
	reg.UnregisterSender("sid-aaa")
	reg.Remove("sid-aaa")

	// Buffer must still hold 5 entries (transient disconnect, not logout).
	if got := buf.Len(logicalID); got != 5 {
		t.Fatalf("buffer after disconnect = %d, want 5 (buffer must survive transient close)", got)
	}

	// Step 4: sid-bbb connects with the same cookie.
	csB := &captureSender{}
	addSessionWithLogicalID(t, auth, reg, "sid-bbb", cookieHash, TransportWebSocket, csB)

	// Verify both share the same LogicalID.
	lidB, ok := reg.LogicalID("sid-bbb")
	if !ok || lidB != logicalID {
		t.Fatalf("sid-bbb LogicalID = %q, want %q", lidB, logicalID)
	}

	// Step 5: full replay (X-Last-Sequence: 0).
	h := http.Header{}
	h.Set(HeaderLastSequence, "0")
	replay := res.Resume("sid-bbb", h)

	if len(replay) != 5 {
		t.Fatalf("replay count = %d, want 5 (AC-BR-AMEND-004 full replay)", len(replay))
	}
	for i, msg := range replay {
		want := uint64(i + 1)
		if msg.Sequence != want {
			t.Fatalf("replay[%d].Sequence = %d, want %d", i, msg.Sequence, want)
		}
	}
}

// TestCrossConnReplay_PartialReplayLastEventID_WebSocket — AC-BR-AMEND-005:
// partial replay via X-Last-Sequence header.
//
// Scenario identical to full replay but sid-bbb reconnects with
// X-Last-Sequence: 3, expecting only sequences 4 and 5.
func TestCrossConnReplay_PartialReplayLastEventID_WebSocket(t *testing.T) {
	t.Parallel()

	auth, reg, buf, disp, res := newCrossConnHarness(t)

	cookieHash := bytes.Repeat([]byte{0x01}, 32)
	logicalID := auth.DeriveLogicalID(cookieHash, TransportWebSocket)

	csA := &captureSender{}
	addSessionWithLogicalID(t, auth, reg, "sid-aaa", cookieHash, TransportWebSocket, csA)

	// Emit 5 chunks (sequence 1-5).
	for range 5 {
		if _, err := disp.SendOutbound("sid-aaa", OutboundChunk, []byte(`"data"`)); err != nil {
			t.Fatalf("SendOutbound: %v", err)
		}
	}
	if got := buf.Len(logicalID); got != 5 {
		t.Fatalf("buffer setup = %d, want 5", got)
	}

	// sid-aaa disconnects.
	reg.UnregisterSender("sid-aaa")
	reg.Remove("sid-aaa")

	// sid-bbb reconnects, last-known sequence = 3.
	csB := &captureSender{}
	addSessionWithLogicalID(t, auth, reg, "sid-bbb", cookieHash, TransportWebSocket, csB)

	h := http.Header{}
	h.Set(HeaderLastSequence, "3")
	replay := res.Resume("sid-bbb", h)

	if len(replay) != 2 {
		t.Fatalf("partial replay count = %d, want 2 (sequences 4,5)", len(replay))
	}
	if replay[0].Sequence != 4 || replay[1].Sequence != 5 {
		t.Fatalf("partial replay sequences = [%d,%d], want [4,5]",
			replay[0].Sequence, replay[1].Sequence)
	}
}

// TestCrossConnReplay_PartialReplayLastEventID_SSE — AC-BR-AMEND-005 (SSE):
// partial replay via Last-Event-ID header on SSE reconnect.
//
// SSE and WS share the same LogicalID derivation logic but use a different
// transport, so their LogicalIDs are distinct. This test verifies the SSE
// header parsing path through the resumer.
func TestCrossConnReplay_PartialReplayLastEventID_SSE(t *testing.T) {
	t.Parallel()

	auth, reg, buf, disp, res := newCrossConnHarness(t)

	cookieHash := bytes.Repeat([]byte{0x02}, 32)
	logicalID := auth.DeriveLogicalID(cookieHash, TransportSSE)

	csA := &captureSender{}
	addSessionWithLogicalID(t, auth, reg, "sse-aaa", cookieHash, TransportSSE, csA)

	// Emit 4 chunks.
	for range 4 {
		if _, err := disp.SendOutbound("sse-aaa", OutboundChunk, []byte(`"chunk"`)); err != nil {
			t.Fatalf("SendOutbound: %v", err)
		}
	}
	if got := buf.Len(logicalID); got != 4 {
		t.Fatalf("SSE buffer = %d, want 4", got)
	}

	// sse-aaa disconnects.
	reg.UnregisterSender("sse-aaa")
	reg.Remove("sse-aaa")

	// sse-bbb reconnects, browser sends Last-Event-ID: 2.
	csB := &captureSender{}
	addSessionWithLogicalID(t, auth, reg, "sse-bbb", cookieHash, TransportSSE, csB)

	h := http.Header{}
	h.Set(HeaderLastEventID, "2")
	replay := res.Resume("sse-bbb", h)

	if len(replay) != 2 {
		t.Fatalf("SSE partial replay count = %d, want 2 (sequences 3,4)", len(replay))
	}
	if replay[0].Sequence != 3 || replay[1].Sequence != 4 {
		t.Fatalf("SSE partial replay sequences = [%d,%d], want [3,4]",
			replay[0].Sequence, replay[1].Sequence)
	}
}

// TestCrossConnReplay_DifferentTransportDoesNotShareBuffer — cross-transport
// isolation: WS and SSE sessions sharing the same cookie produce different
// LogicalIDs, so their buffers are separate.
//
// This is spec §5.3 Alternative B rejection verification.
func TestCrossConnReplay_DifferentTransportDoesNotShareBuffer(t *testing.T) {
	t.Parallel()

	auth, reg, buf, disp, res := newCrossConnHarness(t)

	cookieHash := bytes.Repeat([]byte{0x03}, 32)
	logicalIDWS := auth.DeriveLogicalID(cookieHash, TransportWebSocket)
	logicalIDSSE := auth.DeriveLogicalID(cookieHash, TransportSSE)

	// Verify the two LogicalIDs are distinct.
	if logicalIDWS == logicalIDSSE {
		t.Fatalf("WS and SSE must produce distinct LogicalIDs for the same cookie")
	}

	// WS tab emits 3 chunks.
	csWS := &captureSender{}
	addSessionWithLogicalID(t, auth, reg, "ws-tab", cookieHash, TransportWebSocket, csWS)
	for range 3 {
		if _, err := disp.SendOutbound("ws-tab", OutboundChunk, []byte(`"ws"`)); err != nil {
			t.Fatalf("WS SendOutbound: %v", err)
		}
	}

	// WS tab disconnects.
	reg.UnregisterSender("ws-tab")
	reg.Remove("ws-tab")

	if got := buf.Len(logicalIDWS); got != 3 {
		t.Fatalf("WS buffer = %d, want 3", got)
	}
	if got := buf.Len(logicalIDSSE); got != 0 {
		t.Fatalf("SSE buffer must be empty: got %d", got)
	}

	// SSE session connects (same cookie, different transport).
	csSSE := &captureSender{}
	addSessionWithLogicalID(t, auth, reg, "sse-tab", cookieHash, TransportSSE, csSSE)

	// SSE resumer must return nothing — WS buffer is under a different LogicalID.
	h := http.Header{}
	h.Set(HeaderLastSequence, "0")
	replay := res.Resume("sse-tab", h)
	if len(replay) != 0 {
		t.Fatalf("cross-transport replay must return 0 messages, got %d", len(replay))
	}
}

// TestCrossConnReplay_FallbackWhenNoLogicalID — resumer falls back to connID
// when the session has no LogicalID (pre-amendment sessions, test fixtures).
// This exercises the REQ-BR-AMEND-004 fallback branch.
func TestCrossConnReplay_FallbackWhenNoLogicalID(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	buf := newOutboundBuffer(clockwork.NewFakeClock())
	res := newResumer(buf, reg)

	// Register a session without a LogicalID.
	const sid = "no-lid"
	if err := reg.Add(WebUISession{ID: sid, Transport: TransportWebSocket, State: SessionStateActive}); err != nil {
		t.Fatalf("registry add: %v", err)
	}
	// Manually append a buffer entry keyed by connID (legacy path).
	buf.Append(mkMsg(sid, 1, "fallback-payload"))
	buf.Append(mkMsg(sid, 2, "fallback-payload"))

	// Resume should fall back to connID key and return both messages.
	h := http.Header{}
	h.Set(HeaderLastSequence, "0")
	replay := res.Resume(sid, h)
	if len(replay) != 2 {
		t.Fatalf("fallback replay = %d, want 2", len(replay))
	}
}
