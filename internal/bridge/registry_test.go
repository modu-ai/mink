// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-001
// AC: AC-BR-001
// M0-T4 — in-memory session registry concurrent-safety contract.

package bridge

import (
	"sort"
	"sync"
	"testing"
	"time"
)

func newTestSession(id string, transport Transport) WebUISession {
	return WebUISession{
		ID:           id,
		CookieHash:   []byte("hash-" + id),
		CSRFHash:     []byte("csrf-" + id),
		Transport:    transport,
		OpenedAt:     time.Now(),
		LastActivity: time.Now(),
		State:        SessionStateOpen,
	}
}

func TestRegistry_AddAndGet(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	s := newTestSession("sess-1", TransportWebSocket)

	if err := r.Add(s); err != nil {
		t.Fatalf("Add(sess-1) returned %v, want nil", err)
	}

	got, ok := r.Get("sess-1")
	if !ok {
		t.Fatalf("Get(sess-1) ok = false, want true")
	}
	if got.ID != "sess-1" || got.Transport != TransportWebSocket {
		t.Errorf("Get returned %+v, want id=sess-1 transport=websocket", got)
	}
}

func TestRegistry_AddDuplicateRejected(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	s := newTestSession("dup", TransportSSE)
	if err := r.Add(s); err != nil {
		t.Fatalf("first Add err = %v, want nil", err)
	}
	if err := r.Add(s); err == nil {
		t.Fatalf("duplicate Add err = nil, want non-nil")
	}
}

func TestRegistry_AddRejectsEmptyID(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	s := newTestSession("", TransportWebSocket)
	if err := r.Add(s); err == nil {
		t.Fatalf("Add(empty ID) err = nil, want non-nil")
	}
}

func TestRegistry_RemoveDeletes(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	_ = r.Add(newTestSession("sess-rm", TransportWebSocket))

	r.Remove("sess-rm")

	if _, ok := r.Get("sess-rm"); ok {
		t.Fatalf("Get after Remove ok = true, want false")
	}
}

func TestRegistry_RemoveMissingNoop(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	r.Remove("does-not-exist") // must not panic
}

type counterCloser struct{ count int }

func (c *counterCloser) Close(_ CloseCode) error { c.count++; return nil }

func TestRegistry_RegisterUnregisterCloser(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	cookieHash := []byte("hash-x")
	_ = r.Add(WebUISession{
		ID: "sx", CookieHash: cookieHash, Transport: TransportWebSocket,
		OpenedAt: time.Now(), LastActivity: time.Now(), State: SessionStateOpen,
	})

	cc := &counterCloser{}
	r.RegisterCloser("sx", cc)
	r.RegisterCloser("", cc)    // no-op (empty id)
	r.RegisterCloser("sx", nil) // no-op (nil closer)

	n := r.CloseSessionsByCookieHash(cookieHash, CloseSessionRevoked)
	if n != 1 || cc.count != 1 {
		t.Errorf("after first close: n=%d count=%d, want 1/1", n, cc.count)
	}

	r.UnregisterCloser("sx")
	n = r.CloseSessionsByCookieHash(cookieHash, CloseSessionRevoked)
	if n != 0 || cc.count != 1 {
		t.Errorf("after unregister: n=%d count=%d, want 0/1", n, cc.count)
	}

	// Empty hash and missing-id paths are no-ops.
	if got := r.CloseSessionsByCookieHash(nil, CloseNormal); got != 0 {
		t.Errorf("CloseSessionsByCookieHash(nil) = %d, want 0", got)
	}
}

func TestRegistry_SnapshotIsCopy(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	_ = r.Add(newTestSession("a", TransportWebSocket))
	_ = r.Add(newTestSession("b", TransportSSE))

	snap := r.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("Snapshot length = %d, want 2", len(snap))
	}

	// Mutating the snapshot must not affect the registry.
	snap[0].State = SessionStateClosed

	// Find the session by id and verify state was preserved internally.
	mutatedID := snap[0].ID
	stored, ok := r.Get(mutatedID)
	if !ok {
		t.Fatalf("Get(%s) ok=false after snapshot mutation", mutatedID)
	}
	if stored.State == SessionStateClosed {
		t.Fatalf("registry state %v leaked through snapshot mutation", stored.State)
	}

	ids := []string{snap[0].ID, snap[1].ID}
	sort.Strings(ids)
	if ids[0] != "a" || ids[1] != "b" {
		t.Errorf("snapshot ids = %v, want [a b]", ids)
	}
}

func TestRegistry_ByteSlicesAreDeepCopied(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	original := WebUISession{
		ID:           "deep-copy",
		CookieHash:   []byte{0xaa, 0xbb, 0xcc},
		CSRFHash:     []byte{0x11, 0x22, 0x33},
		Transport:    TransportWebSocket,
		OpenedAt:     time.Now(),
		LastActivity: time.Now(),
		State:        SessionStateOpen,
	}
	if err := r.Add(original); err != nil {
		t.Fatalf("Add err = %v", err)
	}

	// Mutating the input AFTER Add must not affect registry storage.
	original.CookieHash[0] = 0xff
	original.CSRFHash[0] = 0xff

	got, ok := r.Get("deep-copy")
	if !ok {
		t.Fatalf("Get returned ok=false")
	}
	if got.CookieHash[0] != 0xaa {
		t.Errorf("CookieHash[0] = %#x, want 0xaa (input mutation leaked into Add)", got.CookieHash[0])
	}
	if got.CSRFHash[0] != 0x11 {
		t.Errorf("CSRFHash[0] = %#x, want 0x11 (input mutation leaked into Add)", got.CSRFHash[0])
	}

	// Mutating Get's return must not affect future reads.
	got.CookieHash[0] = 0xee
	got.CSRFHash[0] = 0xee

	again, _ := r.Get("deep-copy")
	if again.CookieHash[0] != 0xaa || again.CSRFHash[0] != 0x11 {
		t.Errorf("Get-mutate-Get: CookieHash[0]=%#x CSRFHash[0]=%#x, want 0xaa/0x11",
			again.CookieHash[0], again.CSRFHash[0])
	}

	// Mutating Snapshot's return must not affect the registry either.
	snap := r.Snapshot()
	snap[0].CookieHash[0] = 0xdd
	final, _ := r.Get("deep-copy")
	if final.CookieHash[0] != 0xaa {
		t.Errorf("Snapshot mutation leaked: CookieHash[0]=%#x, want 0xaa", final.CookieHash[0])
	}
}

func TestRegistry_ConcurrentAddRemove(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	const workers = 32
	const perWorker = 50

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func(wid int) {
			defer wg.Done()
			for i := range perWorker {
				id := sessionID(wid, i)
				_ = r.Add(newTestSession(id, TransportWebSocket))
				if i%3 == 0 {
					r.Remove(id)
				}
				_, _ = r.Get(id)
				_ = r.Snapshot()
			}
		}(w)
	}
	wg.Wait()
}

func sessionID(worker, n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, 0, 16)
	b = append(b, "w-"...)
	b = append(b, hex[worker%16])
	b = append(b, "-"...)
	b = append(b, hex[(n>>4)&0xf], hex[n&0xf])
	return string(b)
}
