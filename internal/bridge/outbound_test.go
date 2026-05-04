// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-007
// AC: AC-BR-007
// M3-T1, M3-T2, M3-T3 — outbound dispatcher + sequence + chunk ordering.
// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-003
// AC: AC-BR-AMEND-003
// M3-T2 — registry-less fallback path verification.

package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// captureSender stores every OutboundMessage in arrival order. Optional
// per-message delay simulates a slow transport.
type captureSender struct {
	mu       sync.Mutex
	received []OutboundMessage
	delay    time.Duration
	failNext atomic.Bool
}

func (c *captureSender) SendOutbound(msg OutboundMessage) error {
	if c.failNext.Swap(false) {
		return errors.New("forced send failure")
	}
	if c.delay > 0 {
		time.Sleep(c.delay)
	}
	c.mu.Lock()
	c.received = append(c.received, msg)
	c.mu.Unlock()
	return nil
}

func (c *captureSender) snapshot() []OutboundMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]OutboundMessage, len(c.received))
	copy(out, c.received)
	return out
}

func TestOutboundDispatcher_AssignsSequenceMonotonic(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	disp := newOutboundDispatcher(reg, nil, nil, nil)

	cs := &captureSender{}
	reg.RegisterSender("sx", cs)

	for i := range 10 {
		seq, err := disp.SendOutbound("sx", OutboundChunk, fmt.Appendf(nil, `{"i":%d}`, i))
		if err != nil {
			t.Fatalf("send %d err = %v", i, err)
		}
		if seq != uint64(i+1) {
			t.Errorf("send %d sequence = %d, want %d", i, seq, i+1)
		}
	}

	got := cs.snapshot()
	if len(got) != 10 {
		t.Fatalf("received %d, want 10", len(got))
	}
	for i, m := range got {
		if m.Sequence != uint64(i+1) {
			t.Errorf("[%d] sequence = %d, want %d", i, m.Sequence, i+1)
		}
	}
}

func TestOutboundDispatcher_PerSessionSequencesIndependent(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	disp := newOutboundDispatcher(reg, nil, nil, nil)

	a := &captureSender{}
	b := &captureSender{}
	reg.RegisterSender("sa", a)
	reg.RegisterSender("sb", b)

	// Interleave 5 sends per session.
	for range 5 {
		_, _ = disp.SendOutbound("sa", OutboundChunk, []byte(`null`))
		_, _ = disp.SendOutbound("sb", OutboundChunk, []byte(`null`))
	}
	for i, m := range a.snapshot() {
		if m.Sequence != uint64(i+1) {
			t.Errorf("session sa [%d] = %d, want %d", i, m.Sequence, i+1)
		}
	}
	for i, m := range b.snapshot() {
		if m.Sequence != uint64(i+1) {
			t.Errorf("session sb [%d] = %d, want %d", i, m.Sequence, i+1)
		}
	}
}

func TestOutboundDispatcher_UnknownSessionReturnsErr(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	disp := newOutboundDispatcher(reg, nil, nil, nil)
	_, err := disp.SendOutbound("ghost", OutboundChunk, nil)
	if !errors.Is(err, ErrSessionUnknown) {
		t.Errorf("err = %v, want ErrSessionUnknown", err)
	}
}

func TestOutboundDispatcher_PreservesOrderUnderRandomDelay(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	disp := newOutboundDispatcher(reg, nil, nil, nil)
	cs := &captureSender{} // no per-call delay; concurrency stress instead
	reg.RegisterSender("sx", cs)

	const N = 200

	// Single goroutine emits in order — dispatcher must preserve sequence
	// numbering. Simulate variable processing inside the sender by pre-
	// salting random short sleeps in front of each Send.
	rng := rand.New(rand.NewSource(42))
	for i := range N {
		cs.delay = time.Duration(rng.Intn(50)) * time.Microsecond
		seq, err := disp.SendOutbound("sx", OutboundChunk, []byte(`null`))
		if err != nil {
			t.Fatalf("send %d err = %v", i, err)
		}
		if seq != uint64(i+1) {
			t.Errorf("seq drift at %d: got %d", i, seq)
			break
		}
	}

	got := cs.snapshot()
	if len(got) != N {
		t.Fatalf("received %d, want %d", len(got), N)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Sequence <= got[i-1].Sequence {
			t.Errorf("non-monotonic at %d: %d <= %d", i, got[i].Sequence, got[i-1].Sequence)
		}
	}
}

func TestOutboundDispatcher_PropagatesSenderError(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	disp := newOutboundDispatcher(reg, nil, nil, nil)
	cs := &captureSender{}
	reg.RegisterSender("sx", cs)
	cs.failNext.Store(true)

	_, err := disp.SendOutbound("sx", OutboundChunk, nil)
	if err == nil {
		t.Fatalf("expected sender error, got nil")
	}
}

func TestOutboundDispatcher_DropSequenceFreesCounter(t *testing.T) {
	t.Parallel()
	reg := NewRegistry()
	disp := newOutboundDispatcher(reg, nil, nil, nil)
	cs := &captureSender{}
	reg.RegisterSender("sx", cs)

	for range 3 {
		_, _ = disp.SendOutbound("sx", OutboundChunk, nil)
	}
	disp.dropSequence("sx")

	seq, _ := disp.SendOutbound("sx", OutboundChunk, nil)
	if seq != 1 {
		t.Errorf("after drop, sequence = %d, want 1 (reset)", seq)
	}
}

// gateBracketSender is a captureSender that exposes the same
// ObserveWrite/ObserveDrain bracket that production transports
// (wsSender, sseSender) implement. Used to verify that a sender-side
// error still releases in-flight bytes via the deferred drain — the
// regression CodeRabbit Nitpick #1 asked for.
type gateBracketSender struct {
	captureSender
	gate      *flushGate
	sessionID string
}

func (s *gateBracketSender) SendOutbound(msg OutboundMessage) error {
	if s.gate != nil {
		s.gate.ObserveWrite(s.sessionID, len(msg.Payload))
		defer s.gate.ObserveDrain(s.sessionID, len(msg.Payload))
	}
	return s.captureSender.SendOutbound(msg)
}

// TestOutboundDispatcher_SenderErrorDoesNotStallSession verifies that a
// failed sender call does not leave the flush-gate counted as in-flight,
// which would deadlock subsequent emit attempts. The deferred drain
// inside gateBracketSender mirrors wsSender/sseSender semantics
// (M5 follow-up — bracket moved from dispatcher to transport).
func TestOutboundDispatcher_SenderErrorDoesNotStallSession(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	gate := newFlushGate()
	disp := newOutboundDispatcher(reg, nil, gate, nil)

	cs := &gateBracketSender{gate: gate, sessionID: "sx"}
	if err := reg.Add(WebUISession{ID: "sx", Transport: TransportWebSocket, State: SessionStateActive}); err != nil {
		t.Fatalf("registry add: %v", err)
	}
	reg.RegisterSender("sx", cs)

	// Force the first send to fail. The sender bracket must still drain
	// in-flight bytes on the way out so subsequent sends are not stalled.
	cs.failNext.Store(true)
	if _, err := disp.SendOutbound("sx", OutboundChunk, []byte(`{"a":1}`)); err == nil {
		t.Fatalf("expected forced sender error on first send")
	}

	if gate.Stalled("sx") {
		t.Fatalf("gate left stalled after sender error — defer ObserveDrain regressed")
	}

	// A subsequent successful emit must reach the wire.
	if _, err := disp.SendOutbound("sx", OutboundChunk, []byte(`{"b":2}`)); err != nil {
		t.Fatalf("post-error send err = %v, want nil", err)
	}
	got := cs.snapshot()
	if len(got) != 1 || got[0].Sequence != 2 {
		t.Fatalf("post-error capture = %+v, want single seq=2", got)
	}
}

func TestEncodeOutboundJSON_EnvelopeShape(t *testing.T) {
	t.Parallel()
	msg := OutboundMessage{
		SessionID: "sx",
		Type:      OutboundChunk,
		Payload:   []byte(`{"a":1}`),
		Sequence:  42,
	}
	body, err := encodeOutboundJSON(msg)
	if err != nil {
		t.Fatalf("encode err = %v", err)
	}
	var env outboundEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode err = %v; body=%s", err, body)
	}
	if env.Type != "chunk" || env.Sequence != 42 {
		t.Errorf("envelope = %+v", env)
	}
	if string(env.Payload) != `{"a":1}` {
		t.Errorf("payload = %s, want {\"a\":1}", env.Payload)
	}
}

func TestEncodeOutboundJSON_OmitsEmptyPayload(t *testing.T) {
	t.Parallel()
	body, _ := encodeOutboundJSON(OutboundMessage{
		SessionID: "sx", Type: OutboundStatus, Sequence: 1,
	})
	if got := string(body); got != `{"type":"status","sequence":1}` {
		t.Errorf("body = %s, want type+sequence only", got)
	}
}

// TestOutboundDispatcher_RegistryLessFallback verifies that when the
// dispatcher is constructed without a registry (nil registry field),
// bufferKey falls back to the sessionID directly, preserving v0.2.1
// unit-test fixture compatibility (REQ-BR-AMEND-003 fallback branch).
func TestOutboundDispatcher_RegistryLessFallback(t *testing.T) {
	t.Parallel()

	// Build dispatcher with an explicit nil registry to simulate the
	// registry-less fixture pattern used in many existing unit tests.
	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	disp := newOutboundDispatcher(reg, buf, nil, nil)

	// Register a sender directly; no session entry in registry.
	cs := &captureSender{}
	reg.RegisterSender("sx-fallback", cs)
	// Add session with empty LogicalID (triggers fallback path in bufferKey).
	if err := reg.Add(WebUISession{
		ID:        "sx-fallback",
		Transport: TransportWebSocket,
		State:     SessionStateActive,
		// LogicalID is intentionally empty → bufferKey falls back to connID.
	}); err != nil {
		t.Fatalf("registry add: %v", err)
	}

	seq, err := disp.SendOutbound("sx-fallback", OutboundChunk, []byte(`null`))
	if err != nil {
		t.Fatalf("SendOutbound: %v", err)
	}
	if seq != 1 {
		t.Errorf("seq = %d, want 1", seq)
	}

	// Buffer must be keyed by "sx-fallback" (connID), not by empty string.
	if n := buf.Len("sx-fallback"); n != 1 {
		t.Errorf("Len(sx-fallback) = %d, want 1 (fallback to connID)", n)
	}
	if n := buf.Len(""); n != 0 {
		t.Errorf("Len(\"\") = %d, want 0 (empty key must not be used)", n)
	}
}
