// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-007
// AC: AC-BR-AMEND-008
// M3-T5 — logout eager-drop hook integration tests.

package bridge

import (
	"sync/atomic"
	"testing"
)

// noopCloser is a SessionCloser that records Close invocations.
type noopCloser struct {
	closeCount atomic.Int64
}

func (c *noopCloser) Close(_ CloseCode) error {
	c.closeCount.Add(1)
	return nil
}

// orderingCloser records the call index of its Close invocation relative
// to a shared atomic counter. Used by TestLogout_OrderingHookBeforeCloser.
type orderingCloser struct {
	counter   *atomic.Int64
	callOrder int64 // set on Close
}

func (c *orderingCloser) Close(_ CloseCode) error {
	c.callOrder = c.counter.Add(1)
	return nil
}

// TestLogout_DropsLogicalBufferEagerly verifies AC-BR-AMEND-008 step 1:
// after CloseSessionsByCookieHash, buffer.Len(L1) must be 0 immediately
// (not waiting for TTL).
func TestLogout_DropsLogicalBufferEagerly(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	disp := newOutboundDispatcher(reg, buf, nil, nil)
	reg.SetLogoutHook(disp.dropLogicalBuffer)

	cookieHash := []byte("cookie-abc")

	ses := WebUISession{
		ID:         "sid-aaa",
		CookieHash: cookieHash,
		Transport:  TransportWebSocket,
		State:      SessionStateActive,
		LogicalID:  "L1",
	}
	if err := reg.Add(ses); err != nil {
		t.Fatalf("registry add: %v", err)
	}
	closer := &noopCloser{}
	reg.RegisterCloser("sid-aaa", closer)
	reg.RegisterSender("sid-aaa", &captureSender{})

	// Emit 5 outbound messages → buffer L1 should have 5 entries.
	for range 5 {
		if _, err := disp.SendOutbound("sid-aaa", OutboundChunk, []byte(`null`)); err != nil {
			t.Fatalf("SendOutbound: %v", err)
		}
	}
	if n := buf.Len("L1"); n != 5 {
		t.Fatalf("pre-logout Len(L1) = %d, want 5", n)
	}

	// Trigger logout.
	reg.CloseSessionsByCookieHash(cookieHash, CloseSessionRevoked)

	// Buffer must be empty immediately (eager drop, not TTL).
	if n := buf.Len("L1"); n != 0 {
		t.Errorf("post-logout Len(L1) = %d, want 0 (eager drop)", n)
	}
	// Closer must have been invoked.
	if c := closer.closeCount.Load(); c != 1 {
		t.Errorf("closer invoked %d times, want 1", c)
	}
}

// TestLogout_ResetsSequenceCounter verifies AC-BR-AMEND-008 step 5:
// after logout, a new connection with the same cookie+transport starts
// sequence from 1 (counter was reset).
func TestLogout_ResetsSequenceCounter(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	disp := newOutboundDispatcher(reg, buf, nil, nil)
	reg.SetLogoutHook(disp.dropLogicalBuffer)

	cookieHash := []byte("cookie-reset")

	ses := WebUISession{
		ID:         "sid-old",
		CookieHash: cookieHash,
		Transport:  TransportWebSocket,
		State:      SessionStateActive,
		LogicalID:  "L1",
	}
	if err := reg.Add(ses); err != nil {
		t.Fatalf("registry add old: %v", err)
	}
	reg.RegisterCloser("sid-old", &noopCloser{})
	reg.RegisterSender("sid-old", &captureSender{})

	// Emit 5 messages → sequences 1..5 under L1.
	for range 5 {
		if _, err := disp.SendOutbound("sid-old", OutboundChunk, []byte(`null`)); err != nil {
			t.Fatalf("SendOutbound old: %v", err)
		}
	}

	// Logout.
	reg.CloseSessionsByCookieHash(cookieHash, CloseSessionRevoked)
	reg.Remove("sid-old")

	// Simulate re-login: new session with same cookie+transport → same LogicalID.
	sesNew := WebUISession{
		ID:         "sid-new",
		CookieHash: cookieHash,
		Transport:  TransportWebSocket,
		State:      SessionStateActive,
		LogicalID:  "L1", // same logical ID (same cookie + transport)
	}
	if err := reg.Add(sesNew); err != nil {
		t.Fatalf("registry add new: %v", err)
	}
	reg.RegisterSender("sid-new", &captureSender{})

	// First emit after re-login must return sequence 1 (counter reset).
	seq, err := disp.SendOutbound("sid-new", OutboundChunk, []byte(`null`))
	if err != nil {
		t.Fatalf("SendOutbound new: %v", err)
	}
	if seq != 1 {
		t.Errorf("post-logout first sequence = %d, want 1 (counter reset)", seq)
	}
}

// TestLogout_PreventsCrossSessionReplay verifies AC-BR-AMEND-008 step 4:
// after logout, a new connection with the same cookie cannot replay old
// buffer entries (buffer was eagerly dropped).
func TestLogout_PreventsCrossSessionReplay(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	disp := newOutboundDispatcher(reg, buf, nil, nil)
	reg.SetLogoutHook(disp.dropLogicalBuffer)

	cookieHash := []byte("cookie-replay-prevent")

	ses := WebUISession{
		ID:         "sid-before-logout",
		CookieHash: cookieHash,
		Transport:  TransportWebSocket,
		State:      SessionStateActive,
		LogicalID:  "L2",
	}
	if err := reg.Add(ses); err != nil {
		t.Fatalf("registry add: %v", err)
	}
	reg.RegisterCloser("sid-before-logout", &noopCloser{})
	reg.RegisterSender("sid-before-logout", &captureSender{})

	// Emit 3 messages → L2 has 3 entries.
	for range 3 {
		if _, err := disp.SendOutbound("sid-before-logout", OutboundChunk, []byte(`"data"`)); err != nil {
			t.Fatalf("SendOutbound pre-logout: %v", err)
		}
	}

	// Logout.
	reg.CloseSessionsByCookieHash(cookieHash, CloseSessionRevoked)
	reg.Remove("sid-before-logout")

	// New connection after logout — buffer L2 must be empty.
	sesAfter := WebUISession{
		ID:         "sid-after-logout",
		CookieHash: cookieHash,
		Transport:  TransportWebSocket,
		State:      SessionStateActive,
		LogicalID:  "L2",
	}
	if err := reg.Add(sesAfter); err != nil {
		t.Fatalf("registry add after: %v", err)
	}

	// Replay must return zero messages (no oracle for invalidated session).
	replayed := buf.Replay("L2", 0)
	if len(replayed) != 0 {
		t.Errorf("post-logout Replay(L2, 0) = %d entries, want 0 (security: eager drop)", len(replayed))
	}
}

// TestLogout_OrderingHookBeforeCloser verifies AC-BR-AMEND-008 step 1:
// the logout hook (buffer drop) MUST fire before transport closers.
// An atomic counter records the order: hook gets call-index 1, closer gets 2.
func TestLogout_OrderingHookBeforeCloser(t *testing.T) {
	t.Parallel()

	callOrder := &atomic.Int64{}
	hookOrder := int64(0)

	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	disp := newOutboundDispatcher(reg, buf, nil, nil)

	// Wrap dropLogicalBuffer with ordering tracking.
	reg.SetLogoutHook(func(logicalID string) {
		disp.dropLogicalBuffer(logicalID)
		hookOrder = callOrder.Add(1) // record when hook fires
	})

	cookieHash := []byte("cookie-order")
	ses := WebUISession{
		ID:         "sid-order",
		CookieHash: cookieHash,
		Transport:  TransportWebSocket,
		State:      SessionStateActive,
		LogicalID:  "L3",
	}
	if err := reg.Add(ses); err != nil {
		t.Fatalf("registry add: %v", err)
	}

	closer := &orderingCloser{counter: callOrder}
	reg.RegisterCloser("sid-order", closer)

	// Emit one message so the hook has something to drop.
	reg.RegisterSender("sid-order", &captureSender{})
	if _, err := disp.SendOutbound("sid-order", OutboundChunk, []byte(`null`)); err != nil {
		t.Fatalf("SendOutbound: %v", err)
	}

	reg.CloseSessionsByCookieHash(cookieHash, CloseSessionRevoked)

	// hook must have fired first (order 1), closer second (order 2).
	if hookOrder != 1 {
		t.Errorf("hook call order = %d, want 1 (must fire before closer)", hookOrder)
	}
	if closer.callOrder != 2 {
		t.Errorf("closer call order = %d, want 2 (must fire after hook)", closer.callOrder)
	}
}

// TestLogout_MultipleSessionsSameLogicalID verifies that when two connIDs
// share the same LogicalID, a single logout drops the buffer exactly once
// and both transport closers are invoked.
func TestLogout_MultipleSessionsSameLogicalID(t *testing.T) {
	t.Parallel()

	reg := NewRegistry()
	buf := newOutboundBuffer(nil)
	disp := newOutboundDispatcher(reg, buf, nil, nil)

	hookCallCount := &atomic.Int64{}
	reg.SetLogoutHook(func(logicalID string) {
		hookCallCount.Add(1)
		disp.dropLogicalBuffer(logicalID)
	})

	cookieHash := []byte("cookie-multi")
	for _, id := range []string{"sid-tab-a", "sid-tab-b"} {
		s := WebUISession{
			ID:         id,
			CookieHash: cookieHash,
			Transport:  TransportWebSocket,
			State:      SessionStateActive,
			LogicalID:  "L4",
		}
		if err := reg.Add(s); err != nil {
			t.Fatalf("registry add %s: %v", id, err)
		}
		reg.RegisterCloser(id, &noopCloser{})
		reg.RegisterSender(id, &captureSender{})
	}

	// Emit 3 messages (each from different connID).
	for _, id := range []string{"sid-tab-a", "sid-tab-b", "sid-tab-a"} {
		if _, err := disp.SendOutbound(id, OutboundChunk, []byte(`null`)); err != nil {
			t.Fatalf("SendOutbound(%s): %v", id, err)
		}
	}
	if n := buf.Len("L4"); n != 3 {
		t.Fatalf("pre-logout Len(L4) = %d, want 3", n)
	}

	invoked := reg.CloseSessionsByCookieHash(cookieHash, CloseSessionRevoked)

	// Both closers must have been invoked.
	if invoked != 2 {
		t.Errorf("CloseSessionsByCookieHash returned %d, want 2", invoked)
	}
	// Buffer must be empty.
	if n := buf.Len("L4"); n != 0 {
		t.Errorf("post-logout Len(L4) = %d, want 0", n)
	}
	// Hook must have been called exactly once (one unique LogicalID = "L4").
	if c := hookCallCount.Load(); c != 1 {
		t.Errorf("hook call count = %d, want 1 (one unique LogicalID)", c)
	}
}
