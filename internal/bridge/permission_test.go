// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-008
// AC: AC-BR-008
// M3-T4 — permission roundtrip + 60s default-deny timeout.

package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

// permissionTestStack returns the wired components plus a captureSender so
// tests can inspect outbound traffic.
func permissionTestStack(t *testing.T) (*captureSender, *permissionRequester, *permissionStore, clockwork.FakeClock) {
	t.Helper()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	reg := NewRegistry()
	disp := newOutboundDispatcher(reg, nil, nil)
	cs := &captureSender{}
	reg.RegisterSender("sx", cs)
	store := newPermissionStore(clk)
	req := newPermissionRequester(store, disp, PermissionTimeout)
	return cs, req, store, clk
}

func TestRequestPermission_Granted(t *testing.T) {
	t.Parallel()
	cs, req, _, _ := permissionTestStack(t)

	resultCh := make(chan struct {
		ok  bool
		err error
	}, 1)
	go func() {
		ok, err := req.Request(context.Background(), "sx", []byte(`{"reason":"open file"}`))
		resultCh <- struct {
			ok  bool
			err error
		}{ok, err}
	}()

	// Wait until the outbound permission_request has been emitted, then
	// extract the request_id and resolve it.
	deadline := time.Now().Add(2 * time.Second)
	var id string
	for time.Now().Before(deadline) {
		snap := cs.snapshot()
		if len(snap) > 0 {
			var env permissionRequestEnvelope
			if err := json.Unmarshal(snap[0].Payload, &env); err == nil {
				id = env.RequestID
				break
			}
		}
		time.Sleep(time.Millisecond)
	}
	if id == "" {
		t.Fatalf("did not observe outbound permission_request envelope")
	}
	if err := req.HandleInboundPermissionResponse([]byte(
		`{"request_id":"` + id + `","granted":true}`)); err != nil {
		t.Fatalf("HandleInbound err = %v", err)
	}

	select {
	case r := <-resultCh:
		if !r.ok || r.err != nil {
			t.Errorf("ok=%v err=%v, want true / nil", r.ok, r.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Request did not return after grant")
	}
}

func TestRequestPermission_Denied(t *testing.T) {
	t.Parallel()
	cs, req, _, _ := permissionTestStack(t)

	resultCh := make(chan struct {
		ok  bool
		err error
	}, 1)
	go func() {
		ok, err := req.Request(context.Background(), "sx", nil)
		resultCh <- struct {
			ok  bool
			err error
		}{ok, err}
	}()

	deadline := time.Now().Add(2 * time.Second)
	var id string
	for time.Now().Before(deadline) {
		snap := cs.snapshot()
		if len(snap) > 0 {
			var env permissionRequestEnvelope
			_ = json.Unmarshal(snap[0].Payload, &env)
			id = env.RequestID
			break
		}
		time.Sleep(time.Millisecond)
	}
	if id == "" {
		t.Fatalf("no outbound observed")
	}
	_ = req.HandleInboundPermissionResponse([]byte(
		`{"request_id":"` + id + `","granted":false}`))

	r := <-resultCh
	if r.ok {
		t.Errorf("ok = true, want false")
	}
	if !errors.Is(r.err, ErrPermissionDenied) {
		t.Errorf("err = %v, want ErrPermissionDenied", r.err)
	}
}

func TestRequestPermission_TimeoutEmitsStatusAndDenies(t *testing.T) {
	t.Parallel()
	cs, req, store, clk := permissionTestStack(t)

	resultCh := make(chan struct {
		ok  bool
		err error
	}, 1)
	go func() {
		ok, err := req.Request(context.Background(), "sx", nil)
		resultCh <- struct {
			ok  bool
			err error
		}{ok, err}
	}()

	// Wait until pending count == 1 (the goroutine has called Register).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && store.pendingCount() != 1 {
		time.Sleep(time.Millisecond)
	}
	// Wait until the outbound permission_request has hit the captureSender.
	for time.Now().Before(deadline) && len(cs.snapshot()) < 1 {
		time.Sleep(time.Millisecond)
	}
	// Now the timer is armed; advance the fake clock past the timeout.
	// clockwork.FakeClock.BlockUntil ensures the goroutine is parked on the
	// timer channel before we Advance.
	clk.BlockUntil(1)
	clk.Advance(PermissionTimeout + time.Second)

	r := <-resultCh
	if r.ok {
		t.Errorf("ok = true, want false (default-deny)")
	}
	if !errors.Is(r.err, ErrPermissionTimeout) {
		t.Errorf("err = %v, want ErrPermissionTimeout", r.err)
	}

	// Outbound must contain TWO messages: the original
	// permission_request + a permission_timeout status.
	got := cs.snapshot()
	if len(got) < 2 {
		t.Fatalf("outbound count = %d, want >=2 (request + status)", len(got))
	}
	last := got[len(got)-1]
	if last.Type != OutboundStatus {
		t.Errorf("last outbound type = %s, want %s", last.Type, OutboundStatus)
	}
	var status permissionTimeoutPayload
	if err := json.Unmarshal(last.Payload, &status); err != nil {
		t.Fatalf("status payload decode err = %v", err)
	}
	if status.Code != "permission_timeout" {
		t.Errorf("status.code = %q, want permission_timeout", status.Code)
	}
}

func TestRequestPermission_ContextCancelReturnsErr(t *testing.T) {
	t.Parallel()
	_, req, _, _ := permissionTestStack(t)

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := req.Request(ctx, "sx", nil)
		resultCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Request did not return after ctx cancel")
	}
}

func TestRequestPermission_UnknownSenderReturnsImmediately(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC))
	store := newPermissionStore(clk)
	disp := newOutboundDispatcher(NewRegistry(), nil, nil) // no senders registered
	req := newPermissionRequester(store, disp, PermissionTimeout)

	_, err := req.Request(context.Background(), "ghost", nil)
	if !errors.Is(err, ErrSessionUnknown) {
		t.Errorf("err = %v, want ErrSessionUnknown", err)
	}
	// Pending entry must have been dropped.
	if n := store.pendingCount(); n != 0 {
		t.Errorf("pendingCount = %d, want 0", n)
	}
}

func TestPermissionStore_ResolveUnknownIsNoop(t *testing.T) {
	t.Parallel()
	store := newPermissionStore(nil)
	if ok := store.resolve("nope", true); ok {
		t.Errorf("resolve unknown id = true, want false")
	}
}

func TestHandleInboundPermissionResponse_Malformed(t *testing.T) {
	t.Parallel()
	_, req, _, _ := permissionTestStack(t)
	if err := req.HandleInboundPermissionResponse([]byte(`{not_json`)); err == nil {
		t.Errorf("err = nil, want malformed")
	}
	if err := req.HandleInboundPermissionResponse([]byte(`{"granted":true}`)); err == nil {
		t.Errorf("err = nil, want missing request_id")
	}
}

func TestHandleInboundPermissionResponse_LateDeliveryDropped(t *testing.T) {
	t.Parallel()
	_, req, _, _ := permissionTestStack(t)
	// Resolve a non-existent ID — must return nil, not error.
	err := req.HandleInboundPermissionResponse([]byte(
		`{"request_id":"unknown-id","granted":true}`))
	if err != nil {
		t.Errorf("err = %v, want nil (idempotent late delivery)", err)
	}
}
