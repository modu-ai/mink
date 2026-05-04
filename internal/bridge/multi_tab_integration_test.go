// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-005, REQ-BR-AMEND-006
// AC: AC-BR-AMEND-006, AC-BR-AMEND-007
// M4-T2 — multi-tab integration tests.
//
// These tests verify the "buffer share + emit single" semantics when two or
// more browser tabs share the same (CookieHash, Transport) → LogicalID.
//
// Key invariants tested:
//  - Buffer keyed by LogicalID (shared across all tabs).
//  - Each SendOutbound call emits only to the named connID's sender.
//  - Sibling tab replay works: reconnecting tab receives messages emitted
//    by its sibling while it was disconnected.
//  - Concurrent emits are race-free and produce strictly monotonic sequences.

package bridge

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/jonboulle/clockwork"
)

// newMultiTabHarness constructs the shared harness for multi-tab tests.
// Returns auth, registry, buffer, dispatcher, and resumer — all wired
// together in the same way as server.New.
func newMultiTabHarness(t *testing.T) (
	auth *Authenticator,
	reg *Registry,
	buf *outboundBuffer,
	disp *outboundDispatcher,
	res *resumer,
) {
	t.Helper()
	var err error
	auth, err = NewAuthenticator(AuthConfig{HMACSecret: bytes.Repeat([]byte("m"), 32)})
	if err != nil {
		t.Fatalf("new authenticator: %v", err)
	}
	reg = NewRegistry()
	buf = newOutboundBuffer(clockwork.NewFakeClock())
	gate := newFlushGate()
	disp = newOutboundDispatcher(reg, buf, gate, nil)
	reg.SetLogoutHook(disp.dropLogicalBuffer)
	res = newResumer(buf, reg)
	return
}

// addTab registers a session for a given connID and returns its sender.
// Both sender and closer are registered. Returns the sender for assertions.
func addTab(
	t *testing.T,
	auth *Authenticator,
	reg *Registry,
	connID string,
	cookieHash []byte,
	transport Transport,
) *captureSender {
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
	cs := &captureSender{}
	reg.RegisterSender(connID, cs)
	return cs
}

// TestMultiTab_BufferShareEmitSingle — AC-BR-AMEND-006: multi-tab semantics.
//
// Scenario:
//  1. Two tabs (sid-aaa, sid-bbb) connect with the same cookie via WS → same L1.
//  2. dispatcher.SendOutbound("sid-aaa", payloadA) → buffer L1 gains seq 1.
//     sid-aaa receives payloadA. sid-bbb does NOT receive payloadA.
//  3. dispatcher.SendOutbound("sid-bbb", payloadB) → buffer L1 gains seq 2.
//     sid-bbb receives payloadB. sid-aaa does NOT receive payloadB.
//  4. Buffer L1 contains exactly 2 entries in sequence order.
//  5. sid-aaa closes. sid-ccc reconnects (same cookie) → resumer returns both.
func TestMultiTab_BufferShareEmitSingle(t *testing.T) {
	t.Parallel()

	auth, reg, buf, disp, res := newMultiTabHarness(t)

	cookie := make([]byte, 32) // all-zero cookie
	logicalID := auth.DeriveLogicalID(cookie, TransportWebSocket)

	csA := addTab(t, auth, reg, "sid-aaa", cookie, TransportWebSocket)
	csB := addTab(t, auth, reg, "sid-bbb", cookie, TransportWebSocket)

	payloadA := []byte(`"payload-a"`)
	payloadB := []byte(`"payload-b"`)

	// Emit to sid-aaa.
	seqA, err := disp.SendOutbound("sid-aaa", OutboundChunk, payloadA)
	if err != nil {
		t.Fatalf("emit to sid-aaa: %v", err)
	}

	// Emit to sid-bbb.
	seqB, err := disp.SendOutbound("sid-bbb", OutboundChunk, payloadB)
	if err != nil {
		t.Fatalf("emit to sid-bbb: %v", err)
	}

	// Sequences must be monotonic per LogicalID.
	if seqA >= seqB {
		t.Fatalf("sequences not monotonic: seqA=%d, seqB=%d", seqA, seqB)
	}

	// sid-aaa received payloadA only.
	snapA := csA.snapshot()
	if len(snapA) != 1 {
		t.Fatalf("sid-aaa emit count = %d, want 1", len(snapA))
	}
	if string(snapA[0].Payload) != string(payloadA) {
		t.Fatalf("sid-aaa payload = %q, want %q", snapA[0].Payload, payloadA)
	}

	// sid-bbb received payloadB only.
	snapB := csB.snapshot()
	if len(snapB) != 1 {
		t.Fatalf("sid-bbb emit count = %d, want 1", len(snapB))
	}
	if string(snapB[0].Payload) != string(payloadB) {
		t.Fatalf("sid-bbb payload = %q, want %q", snapB[0].Payload, payloadB)
	}

	// Buffer L1 holds exactly 2 entries.
	if got := buf.Len(logicalID); got != 2 {
		t.Fatalf("buffer[%s] = %d, want 2", logicalID, got)
	}

	// sid-aaa closes (transient disconnect). Buffer intact.
	reg.UnregisterSender("sid-aaa")
	reg.Remove("sid-aaa")

	if got := buf.Len(logicalID); got != 2 {
		t.Fatalf("buffer after sid-aaa close = %d, want 2", got)
	}

	// sid-ccc reconnects (same cookie) → resume must return both payloads.
	csC := addTab(t, auth, reg, "sid-ccc", cookie, TransportWebSocket)
	_ = csC

	h := mkResumeHeader(0)
	replay := res.Resume("sid-ccc", h)
	if len(replay) != 2 {
		t.Fatalf("replay for sid-ccc = %d, want 2 (payloadA + payloadB)", len(replay))
	}
	// Replay order must match sequence (payloadA first, then payloadB).
	if replay[0].Sequence != seqA || replay[1].Sequence != seqB {
		t.Fatalf("replay sequences = [%d,%d], want [%d,%d]",
			replay[0].Sequence, replay[1].Sequence, seqA, seqB)
	}
}

// TestMultiTab_SiblingResumeAfterClose — sibling tab pattern from AC-BR-AMEND-006.
//
// Scenario:
//  1. Tab-A and Tab-B both active, sharing LogicalID L1.
//  2. Tab-A emits 3 chunks (seq 1-3). Tab-B emits nothing.
//  3. Tab-A closes (transient disconnect).
//  4. Tab-B is still active.
//  5. Tab-C reconnects (same cookie) with X-Last-Sequence: 0.
//  6. Tab-C receives Tab-A's 3 chunks (via buffer L1).
func TestMultiTab_SiblingResumeAfterClose(t *testing.T) {
	t.Parallel()

	auth, reg, buf, disp, res := newMultiTabHarness(t)

	cookie := bytes.Repeat([]byte{0xAA}, 32)
	logicalID := auth.DeriveLogicalID(cookie, TransportWebSocket)

	csA := addTab(t, auth, reg, "tab-a", cookie, TransportWebSocket)
	csB := addTab(t, auth, reg, "tab-b", cookie, TransportWebSocket)

	// Tab-A emits 3 chunks.
	for i := range 3 {
		if _, err := disp.SendOutbound("tab-a", OutboundChunk, []byte(fmt.Sprintf(`"a-%d"`, i))); err != nil {
			t.Fatalf("tab-a emit[%d]: %v", i, err)
		}
	}

	// sid-b received nothing (only tab-a was named in SendOutbound).
	if got := len(csB.snapshot()); got != 0 {
		t.Fatalf("tab-b received %d messages, want 0 (emit-single invariant)", got)
	}
	// tab-a received 3 live emits.
	if got := len(csA.snapshot()); got != 3 {
		t.Fatalf("tab-a live emits = %d, want 3", got)
	}

	// Buffer L1 has 3 entries.
	if got := buf.Len(logicalID); got != 3 {
		t.Fatalf("buffer[%s] = %d, want 3", logicalID, got)
	}

	// Tab-A closes (transient disconnect).
	reg.UnregisterSender("tab-a")
	reg.Remove("tab-a")

	// Tab-B is still active; buffer is intact.
	if got := buf.Len(logicalID); got != 3 {
		t.Fatalf("buffer after tab-a close = %d, want 3", got)
	}

	// Tab-C reconnects (same cookie, new connID).
	csC := addTab(t, auth, reg, "tab-c", cookie, TransportWebSocket)
	_ = csC

	h := mkResumeHeader(0)
	replay := res.Resume("tab-c", h)
	if len(replay) != 3 {
		t.Fatalf("tab-c replay = %d, want 3 (tab-a's history via shared L1)", len(replay))
	}
	for i, msg := range replay {
		want := uint64(i + 1)
		if msg.Sequence != want {
			t.Fatalf("replay[%d].Sequence = %d, want %d", i, msg.Sequence, want)
		}
	}
}

// TestMultiTab_RaceFreeUnderConcurrentEmit — sequence monotonicity under
// concurrent emit from two goroutines targeting different connIDs that share
// the same LogicalID. Verifies AC-BR-AMEND-007 at the integration level.
//
// Each goroutine calls SendOutbound 50 times; total = 100 messages. The
// buffer must contain exactly sequences 1..100 with no gaps and no duplicates.
func TestMultiTab_RaceFreeUnderConcurrentEmit(t *testing.T) {
	t.Parallel()

	auth, reg, buf, disp, _ := newMultiTabHarness(t)

	cookie := bytes.Repeat([]byte{0xBB}, 32)
	logicalID := auth.DeriveLogicalID(cookie, TransportWebSocket)

	// Both tabs register (sends happen concurrently).
	addTab(t, auth, reg, "race-a", cookie, TransportWebSocket)
	addTab(t, auth, reg, "race-b", cookie, TransportWebSocket)

	const perTab = 50
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for range perTab {
			if _, err := disp.SendOutbound("race-a", OutboundChunk, []byte(`"ra"`)); err != nil {
				t.Errorf("race-a SendOutbound: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for range perTab {
			if _, err := disp.SendOutbound("race-b", OutboundChunk, []byte(`"rb"`)); err != nil {
				t.Errorf("race-b SendOutbound: %v", err)
				return
			}
		}
	}()
	wg.Wait()

	// Collect all buffered sequences.
	msgs := buf.Replay(logicalID, 0)
	if len(msgs) != 2*perTab {
		t.Fatalf("buffer message count = %d, want %d", len(msgs), 2*perTab)
	}

	// Build a set of sequences and verify no duplicates.
	seen := make(map[uint64]struct{}, 2*perTab)
	for _, m := range msgs {
		if _, dup := seen[m.Sequence]; dup {
			t.Fatalf("duplicate sequence %d in buffer", m.Sequence)
		}
		seen[m.Sequence] = struct{}{}
	}

	// Verify the set is {1, ..., 100} with no gaps.
	for i := uint64(1); i <= 2*perTab; i++ {
		if _, ok := seen[i]; !ok {
			t.Fatalf("sequence gap: %d missing from buffer", i)
		}
	}
}

// mkResumeHeader returns an http.Header with X-Last-Sequence set to seq.
func mkResumeHeader(seq uint64) http.Header {
	h := http.Header{}
	h.Set(HeaderLastSequence, fmt.Sprintf("%d", seq))
	return h
}
