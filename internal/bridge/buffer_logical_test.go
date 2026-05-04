// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-003, REQ-BR-AMEND-006
// AC: AC-BR-AMEND-003, AC-BR-AMEND-007
// M3-T4 — buffer LogicalID keying + dispatcher sequence monotonic race test.

package bridge

import (
	"encoding/json"
	"sync"
	"testing"
)

// TestBuffer_AppendByLogicalID verifies that Append uses msg.SessionID as
// the buffer key, and that replaying the same key returns the message.
// Since AMEND-001 M3 the dispatcher sets msg.SessionID to the LogicalID
// before calling Append, so passing "L1" as SessionID simulates that path.
func TestBuffer_AppendByLogicalID(t *testing.T) {
	t.Parallel()

	buf := newOutboundBuffer(nil)
	msg := OutboundMessage{
		SessionID: "L1",
		Type:      OutboundChunk,
		Payload:   []byte(`{"i":0}`),
		Sequence:  1,
	}
	buf.Append(msg)

	got := buf.Replay("L1", 0)
	if len(got) != 1 {
		t.Fatalf("Replay(L1, 0) = %d entries, want 1", len(got))
	}
	if got[0].Sequence != 1 {
		t.Errorf("got[0].Sequence = %d, want 1", got[0].Sequence)
	}
	// Confirm the connID bucket is empty (no cross-keying).
	if n := buf.Len("connID-aaa"); n != 0 {
		t.Errorf("Len(connID-aaa) = %d, want 0 (LogicalID-keyed only)", n)
	}
}

// TestBuffer_ReplayByLogicalID verifies partial replay: append 5 messages
// keyed "L1", then Replay("L1", 3) returns only messages with Sequence > 3.
func TestBuffer_ReplayByLogicalID(t *testing.T) {
	t.Parallel()

	buf := newOutboundBuffer(nil)
	for i := uint64(1); i <= 5; i++ {
		buf.Append(OutboundMessage{
			SessionID: "L1",
			Type:      OutboundChunk,
			Payload:   []byte(`null`),
			Sequence:  i,
		})
	}

	// Full replay.
	if got := buf.Replay("L1", 0); len(got) != 5 {
		t.Fatalf("Replay(L1, 0) = %d, want 5", len(got))
	}

	// Partial replay from sequence 3.
	got := buf.Replay("L1", 3)
	if len(got) != 2 {
		t.Fatalf("Replay(L1, 3) = %d, want 2 (seq 4,5)", len(got))
	}
	if got[0].Sequence != 4 || got[1].Sequence != 5 {
		t.Errorf("partial replay seqs = %d,%d, want 4,5", got[0].Sequence, got[1].Sequence)
	}
}

// TestBuffer_FallbackToSessionIDWhenLogicalIDEmpty verifies that a dispatcher
// constructed without a registry (nil registry) falls back to using the
// connID (sessionID) directly as the buffer key. This preserves v0.2.1
// compatibility for unit-test fixtures.
func TestBuffer_FallbackToSessionIDWhenLogicalIDEmpty(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	// Dispatcher with registry but session has no LogicalID registered.
	disp := newOutboundDispatcher(reg, buf, nil, nil)

	cs := &captureSender{}
	reg.RegisterSender("conn-no-logical", cs)
	if err := reg.Add(WebUISession{
		ID:        "conn-no-logical",
		Transport: TransportWebSocket,
		State:     SessionStateActive,
		// LogicalID intentionally empty — simulates pre-amendment session.
	}); err != nil {
		t.Fatalf("registry add: %v", err)
	}

	seq, err := disp.SendOutbound("conn-no-logical", OutboundChunk, []byte(`{}`))
	if err != nil {
		t.Fatalf("SendOutbound err = %v", err)
	}
	if seq != 1 {
		t.Errorf("seq = %d, want 1", seq)
	}

	// Buffer should be keyed by connID (fallback), not by an empty LogicalID.
	if n := buf.Len("conn-no-logical"); n != 1 {
		t.Errorf("Len(conn-no-logical) = %d, want 1 (fallback connID key)", n)
	}
	// Empty-string key should be empty.
	if n := buf.Len(""); n != 0 {
		t.Errorf("Len(\"\") = %d, want 0", n)
	}
}

// TestDispatcher_SequenceMonotonicPerLogicalID is the AC-BR-AMEND-007 race
// test. Two connIDs (sid-aaa, sid-bbb) share the same LogicalID "L1".
// Each goroutine calls SendOutbound 100 times concurrently (200 total).
// The collected sequence set from buffer.Replay("L1", 0) must be exactly
// {1, 2, ..., 200} — no gaps, no duplicates.
//
// Run with: go test -race -count=10 -run TestDispatcher_SequenceMonotonicPerLogicalID
func TestDispatcher_SequenceMonotonicPerLogicalID(t *testing.T) {
	t.Parallel()

	const N = 100
	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	disp := newOutboundDispatcher(reg, buf, nil, nil)

	// Register two sessions sharing LogicalID "L1".
	sesA := WebUISession{
		ID:        "sid-aaa",
		Transport: TransportWebSocket,
		State:     SessionStateActive,
		LogicalID: "L1",
	}
	sesB := WebUISession{
		ID:        "sid-bbb",
		Transport: TransportWebSocket,
		State:     SessionStateActive,
		LogicalID: "L1",
	}
	for _, s := range []WebUISession{sesA, sesB} {
		if err := reg.Add(s); err != nil {
			t.Fatalf("registry add %s: %v", s.ID, err)
		}
		reg.RegisterSender(s.ID, &captureSender{})
	}

	var wg sync.WaitGroup
	wg.Add(2)

	emitN := func(connID string) {
		defer wg.Done()
		for range N {
			if _, err := disp.SendOutbound(connID, OutboundChunk, []byte(`null`)); err != nil {
				t.Errorf("SendOutbound(%s) err = %v", connID, err)
			}
		}
	}

	go emitN("sid-aaa")
	go emitN("sid-bbb")
	wg.Wait()

	msgs := buf.Replay("L1", 0)
	if len(msgs) != 2*N {
		t.Fatalf("Replay(L1, 0) = %d entries, want %d", len(msgs), 2*N)
	}

	// Build a set of sequences and verify {1..200}.
	seen := make(map[uint64]int, 2*N)
	for _, m := range msgs {
		seen[m.Sequence]++
	}
	for seq := uint64(1); seq <= 2*N; seq++ {
		if seen[seq] != 1 {
			t.Errorf("sequence %d count = %d, want 1 (no gap, no duplicate)", seq, seen[seq])
		}
	}
}

// TestDispatcher_WireEnvelopeIgnoresSessionIDSwap is the D3 invariant
// test (spec §7.1). It verifies that encodeOutboundJSON produces identical
// bytes regardless of the OutboundMessage.SessionID value, and that the
// resulting JSON keys do NOT include a "session_id" field.
//
// This locks down the invariant that the buffer-keying swap
// (bufKeyMsg.SessionID = logicalID) is invisible on the wire.
func TestDispatcher_WireEnvelopeIgnoresSessionIDSwap(t *testing.T) {
	t.Parallel()

	base := OutboundMessage{
		Type:     OutboundChunk,
		Sequence: 42,
		Payload:  []byte(`"hello"`),
	}

	withConnID := base
	withConnID.SessionID = "conn-xxx"

	withLogicalID := base
	withLogicalID.SessionID = "L1-logical"

	bytesA, err := encodeOutboundJSON(withConnID)
	if err != nil {
		t.Fatalf("encodeOutboundJSON(connID): %v", err)
	}
	bytesB, err := encodeOutboundJSON(withLogicalID)
	if err != nil {
		t.Fatalf("encodeOutboundJSON(logicalID): %v", err)
	}

	// Must be byte-equal: SessionID differences must not affect the envelope.
	if string(bytesA) != string(bytesB) {
		t.Errorf("wire envelopes differ:\n  connID:    %s\n  logicalID: %s", bytesA, bytesB)
	}

	// Decode and verify the JSON keys are exactly {type, sequence, payload}.
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(bytesA, &decoded); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	// session_id must NOT be present (spec §7.1 invariant).
	if _, ok := decoded["session_id"]; ok {
		t.Error("envelope contains session_id — spec §7.1 invariant violated")
	}
	for _, required := range []string{"type", "sequence"} {
		if _, ok := decoded[required]; !ok {
			t.Errorf("envelope missing required field %q", required)
		}
	}
}

// TestDispatcher_BufferKeyedByLogicalIDNotConnID verifies that after a
// SendOutbound call where the session has a LogicalID, the buffer contains
// an entry under the LogicalID bucket and NOT under the connID bucket.
// This directly validates AC-BR-AMEND-003.
func TestDispatcher_BufferKeyedByLogicalIDNotConnID(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	disp := newOutboundDispatcher(reg, buf, nil, nil)

	ses := WebUISession{
		ID:        "sid-aaa",
		Transport: TransportWebSocket,
		State:     SessionStateActive,
		LogicalID: "L1",
	}
	if err := reg.Add(ses); err != nil {
		t.Fatalf("registry add: %v", err)
	}
	reg.RegisterSender("sid-aaa", &captureSender{})

	if _, err := disp.SendOutbound("sid-aaa", OutboundChunk, []byte(`{}`)); err != nil {
		t.Fatalf("SendOutbound: %v", err)
	}

	// Buffer entry must be under "L1" (LogicalID), not "sid-aaa" (connID).
	if n := buf.Len("L1"); n != 1 {
		t.Errorf("Len(L1) = %d, want 1 (LogicalID-keyed)", n)
	}
	if n := buf.Len("sid-aaa"); n != 0 {
		t.Errorf("Len(sid-aaa) = %d, want 0 (connID bucket must be empty)", n)
	}
}
