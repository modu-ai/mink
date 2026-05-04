// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-009, REQ-BR-010, REQ-BR-017
// AC: AC-BR-009, AC-BR-010, AC-BR-015
// M4 integration — dispatcher wired with buffer + flush-gate, end-to-end
// scenarios for write-storm backpressure and tab-background resume.

package bridge

import (
	"net/http"
	"testing"

	"github.com/jonboulle/clockwork"
)

// newM4Dispatcher returns a dispatcher with both buffer and flush-gate
// wired up, mirroring the production server.New construction.
func newM4Dispatcher() (*outboundDispatcher, *outboundBuffer, *flushGate, *Registry) {
	reg := NewRegistry()
	buf := newOutboundBuffer(clockwork.NewFakeClock())
	gate := newFlushGate()
	disp := newOutboundDispatcher(reg, buf, gate, nil)
	return disp, buf, gate, reg
}

func registerSession(t *testing.T, reg *Registry, id string, sender SessionSender) {
	t.Helper()
	if err := reg.Add(WebUISession{ID: id, Transport: TransportWebSocket, State: SessionStateActive}); err != nil {
		t.Fatalf("registry add: %v", err)
	}
	if sender != nil {
		reg.RegisterSender(id, sender)
	}
}

// TestM4_WriteStormStallsThenDrains — simulate a slow transport by
// pushing the gate above the high watermark via direct ObserveWrite (the
// dispatcher itself brackets each emit, so the test must drive the gate
// manually to model concurrent in-flight pressure). While stalled,
// SendOutbound buffers without emitting; after manual ObserveDrain
// crosses the low watermark, emit resumes.
func TestM4_WriteStormStallsThenDrains(t *testing.T) {
	t.Parallel()
	disp, buf, gate, reg := newM4Dispatcher()
	cs := &captureSender{}
	registerSession(t, reg, "s", cs)

	// Pre-stall the gate by simulating outstanding transport writes.
	// 40 frames × 8 KiB = 320 KiB > HighWatermarkBytes=256 KiB.
	payload := make([]byte, 8*1024)
	for range 40 {
		gate.ObserveWrite("s", len(payload))
	}
	if !gate.Stalled("s") {
		t.Fatalf("setup expected stall after manual ObserveWrite")
	}
	stallAtFreeze := gate.Stalls()

	// While stalled, dispatcher SendOutbound must buffer but not emit.
	emitsBeforeBuffered := len(cs.snapshot())
	for range 5 {
		if _, err := disp.SendOutbound("s", OutboundChunk, payload); err != nil {
			t.Fatalf("send-while-stalled err: %v", err)
		}
	}
	if got := len(cs.snapshot()); got != emitsBeforeBuffered {
		t.Fatalf("emits while stalled: got %d, want unchanged %d", got, emitsBeforeBuffered)
	}
	if buf.Len("s") != 5 {
		t.Fatalf("buffer should retain all 5 stalled-phase messages, got %d", buf.Len("s"))
	}

	// Drain: simulate transport flushing the queue.
	for range 40 {
		gate.ObserveDrain("s", len(payload))
	}
	if gate.Stalled("s") {
		t.Fatalf("gate must drain below low watermark after full drain")
	}
	if got := gate.Stalls(); got != stallAtFreeze {
		t.Fatalf("stall counter must not increase during drain: got %d", got)
	}

	// Post-drain emit reaches the wire.
	if _, err := disp.SendOutbound("s", OutboundChunk, payload); err != nil {
		t.Fatalf("post-drain send err: %v", err)
	}
	if got := len(cs.snapshot()); got != emitsBeforeBuffered+1 {
		t.Fatalf("post-drain emit failed: got %d, want %d", got, emitsBeforeBuffered+1)
	}
}

// TestM4_TabBackgroundReconnectReplays — sender unregisters mid-flow
// (tab backgrounded, WebSocket close 1001). Producer keeps sending; on
// reconnect, the resumer hands back exactly the missed chunks in
// sequence with no gaps.
func TestM4_TabBackgroundReconnectReplays(t *testing.T) {
	t.Parallel()
	disp, buf, _, reg := newM4Dispatcher()
	cs := &captureSender{}
	registerSession(t, reg, "s", cs)

	// Two messages delivered live.
	for range 2 {
		if _, err := disp.SendOutbound("s", OutboundChunk, []byte(`"live"`)); err != nil {
			t.Fatalf("live send: %v", err)
		}
	}
	livePhase := cs.snapshot()
	if len(livePhase) != 2 || livePhase[1].Sequence != 2 {
		t.Fatalf("live phase wrong: %+v", livePhase)
	}

	// Tab background: transport unregisters but session stays registered
	// (REQ-BR-017 reconnecting state).
	reg.UnregisterSender("s")

	// 5 outbound chunks queued while disconnected.
	for range 5 {
		seq, err := disp.SendOutbound("s", OutboundChunk, []byte(`"buffered"`))
		if err != nil {
			t.Fatalf("buffered send: %v", err)
		}
		if seq < 3 || seq > 7 {
			t.Fatalf("unexpected sequence during background: %d", seq)
		}
	}
	if got := len(cs.snapshot()); got != 2 {
		t.Fatalf("sender must not receive while unregistered: got %d", got)
	}
	if got := buf.Len("s"); got != 7 {
		t.Fatalf("buffer must retain all 7: got %d", got)
	}

	// Reconnect: re-register sender, ask resumer for messages after the
	// browser's last-known sequence (=2).
	cs2 := &captureSender{}
	reg.RegisterSender("s", cs2)
	r := newResumer(buf, nil)
	h := http.Header{}
	h.Set(HeaderLastSequence, "2")
	replay := r.Resume("s", h)
	if len(replay) != 5 {
		t.Fatalf("replay count: got %d, want 5", len(replay))
	}
	for i, m := range replay {
		want := uint64(i + 3)
		if m.Sequence != want {
			t.Fatalf("replay sequence gap at i=%d: got %d, want %d", i, m.Sequence, want)
		}
	}
}

// TestM4_DispatcherDropClearsBufferAndGate — dropSequence on a session
// must release per-session memory in buffer, gate, and counter map.
func TestM4_DispatcherDropClearsBufferAndGate(t *testing.T) {
	t.Parallel()
	disp, buf, gate, reg := newM4Dispatcher()
	cs := &captureSender{}
	registerSession(t, reg, "s", cs)

	for range 3 {
		_, _ = disp.SendOutbound("s", OutboundChunk, []byte("x"))
	}
	if buf.Len("s") != 3 {
		t.Fatalf("buffer setup expected 3, got %d", buf.Len("s"))
	}

	disp.dropSequence("s")

	if buf.Len("s") != 0 {
		t.Fatalf("buffer not cleared: %d", buf.Len("s"))
	}
	if gate.Stalled("s") {
		t.Fatalf("gate state leaked after drop")
	}
}

// TestM4_GhostSessionStillReturnsErrSessionUnknown — preserve the M3
// contract for sessions that never existed in the registry.
func TestM4_GhostSessionStillReturnsErrSessionUnknown(t *testing.T) {
	t.Parallel()
	disp, _, _, _ := newM4Dispatcher()
	if _, err := disp.SendOutbound("ghost", OutboundChunk, nil); err == nil {
		t.Fatalf("expected ErrSessionUnknown for ghost session")
	}
}
