// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-007
// AC: AC-BR-007
// M3-T1, M3-T2, M3-T3 — outbound dispatcher + sequence + chunk ordering.

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
