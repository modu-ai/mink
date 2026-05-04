// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-009, REQ-BR-017
// AC: AC-BR-009, AC-BR-015
// M4-T1/T2/T3 — buffer behavior under eviction, TTL, and concurrent access.

package bridge

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
)

func mkMsg(sid string, seq uint64, payload string) OutboundMessage {
	return OutboundMessage{
		SessionID: sid,
		Type:      OutboundChunk,
		Payload:   []byte(payload),
		Sequence:  seq,
	}
}

func TestBuffer_AppendAndReplay_FIFO(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	b := newOutboundBuffer(clock)
	sid := "sess-1"

	for i := uint64(1); i <= 5; i++ {
		b.Append(mkMsg(sid, i, "chunk"))
	}

	got := b.Replay(sid, 0)
	if len(got) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(got))
	}
	for i, m := range got {
		if m.Sequence != uint64(i+1) {
			t.Fatalf("FIFO violated: got seq %d at index %d", m.Sequence, i)
		}
	}
}

func TestBuffer_Replay_LastSequenceFilter(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"
	for i := uint64(1); i <= 5; i++ {
		b.Append(mkMsg(sid, i, "x"))
	}

	got := b.Replay(sid, 3)
	if len(got) != 2 {
		t.Fatalf("expected 2 (seq>3), got %d", len(got))
	}
	if got[0].Sequence != 4 || got[1].Sequence != 5 {
		t.Fatalf("unexpected sequences: %+v", got)
	}
}

func TestBuffer_Replay_EmptySession(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	if got := b.Replay("never", 0); got != nil {
		t.Fatalf("expected nil for unknown session, got %+v", got)
	}
}

func TestBuffer_OldestDropOnMessageLimit(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"

	for i := uint64(1); i <= MaxBufferMessages+10; i++ {
		b.Append(mkMsg(sid, i, "y"))
	}

	got := b.Replay(sid, 0)
	if len(got) != MaxBufferMessages {
		t.Fatalf("expected count cap %d, got %d", MaxBufferMessages, len(got))
	}
	// Oldest 10 must be dropped; newest survives.
	if got[0].Sequence != 11 {
		t.Fatalf("oldest-drop broken: head seq=%d", got[0].Sequence)
	}
	if got[len(got)-1].Sequence != uint64(MaxBufferMessages+10) {
		t.Fatalf("tail not preserved: tail seq=%d", got[len(got)-1].Sequence)
	}
}

func TestBuffer_OldestDropOnByteLimit(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"
	// Each payload 1 MiB; after 5 appends total 5 MiB > 4 MiB → at least 1 drop.
	payload := strings.Repeat("a", 1024*1024)
	for i := uint64(1); i <= 5; i++ {
		b.Append(mkMsg(sid, i, payload))
	}

	got := b.Replay(sid, 0)
	if b.Bytes(sid) > MaxBufferBytes {
		t.Fatalf("byte cap exceeded: %d > %d", b.Bytes(sid), MaxBufferBytes)
	}
	if got[0].Sequence == 1 {
		t.Fatalf("expected oldest seq=1 to be evicted, but it survived")
	}
}

func TestBuffer_TTLEvictsOldEntries(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	b := newOutboundBuffer(clock)
	sid := "sess-1"

	b.Append(mkMsg(sid, 1, "old"))
	clock.Advance(BufferTTL + time.Minute)
	b.Append(mkMsg(sid, 2, "new"))

	got := b.Replay(sid, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 after TTL eviction, got %d", len(got))
	}
	if got[0].Sequence != 2 {
		t.Fatalf("expected seq=2 to survive, got %d", got[0].Sequence)
	}
}

func TestBuffer_TTLPreservesEntriesWithinWindow(t *testing.T) {
	t.Parallel()
	clock := clockwork.NewFakeClock()
	b := newOutboundBuffer(clock)
	sid := "sess-1"

	b.Append(mkMsg(sid, 1, "x"))
	clock.Advance(BufferTTL - time.Minute)
	b.Append(mkMsg(sid, 2, "y"))

	got := b.Replay(sid, 0)
	if len(got) != 2 {
		t.Fatalf("entries within TTL must survive: got %d", len(got))
	}
}

func TestBuffer_Drop(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"
	b.Append(mkMsg(sid, 1, "x"))
	b.Drop(sid)
	if b.Len(sid) != 0 {
		t.Fatalf("Drop did not clear queue")
	}
	if b.Bytes(sid) != 0 {
		t.Fatalf("Drop did not clear byte total")
	}
}

func TestBuffer_PerSessionIsolation(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	b.Append(mkMsg("a", 1, "x"))
	b.Append(mkMsg("b", 1, "y"))
	if got := b.Replay("a", 0); len(got) != 1 || got[0].SessionID != "a" {
		t.Fatalf("session a leaked or empty: %+v", got)
	}
	if got := b.Replay("b", 0); len(got) != 1 || got[0].SessionID != "b" {
		t.Fatalf("session b leaked or empty: %+v", got)
	}
}

func TestBuffer_NoSequenceGapsAfterEviction(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"
	for i := uint64(1); i <= MaxBufferMessages+50; i++ {
		b.Append(mkMsg(sid, i, "z"))
	}
	got := b.Replay(sid, 0)
	for i := 1; i < len(got); i++ {
		if got[i].Sequence != got[i-1].Sequence+1 {
			t.Fatalf("sequence gap at %d: prev=%d cur=%d", i, got[i-1].Sequence, got[i].Sequence)
		}
	}
}

func TestBuffer_NoDuplicates(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"
	for i := uint64(1); i <= 100; i++ {
		b.Append(mkMsg(sid, i, "x"))
	}
	got := b.Replay(sid, 0)
	seen := make(map[uint64]struct{}, len(got))
	for _, m := range got {
		if _, dup := seen[m.Sequence]; dup {
			t.Fatalf("duplicate sequence %d", m.Sequence)
		}
		seen[m.Sequence] = struct{}{}
	}
}

func TestBuffer_ConcurrentAppendReplayRaceFree(t *testing.T) {
	t.Parallel()
	b := newOutboundBuffer(clockwork.NewFakeClock())
	sid := "sess-1"

	var wg sync.WaitGroup
	const n = 200
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := uint64(1); i <= n; i++ {
			b.Append(mkMsg(sid, i, "x"))
		}
	}()
	go func() {
		defer wg.Done()
		for range n {
			_ = b.Replay(sid, 0)
		}
	}()
	wg.Wait()

	if b.Len(sid) > MaxBufferMessages {
		t.Fatalf("count cap broken under concurrency: %d", b.Len(sid))
	}
}
