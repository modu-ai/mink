// SPEC: SPEC-GOOSE-BRIDGE-001
// REQ: REQ-BR-009, REQ-BR-017
// AC: AC-BR-009, AC-BR-015
// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-003, REQ-BR-AMEND-006
// AC: AC-BR-AMEND-003, AC-BR-AMEND-007
// M4-T1, M4-T2, M4-T3 — per-session outbound ring buffer with 24h TTL.
// M3-T1 (AMEND-001) — buffer key semantics changed from connID to LogicalID.
//
// Design:
//   - One queue per key (LogicalID since AMEND-001 M3), FIFO ordering
//     matches OutboundMessage.Sequence. Before AMEND-001, the key was the
//     connID (sessionID); it is now the HMAC-derived LogicalID shared by all
//     connIDs with the same (CookieHash, Transport).
//   - Eviction triggers when EITHER total bytes >= MaxBufferBytes (4 MiB)
//     OR queued count >= MaxBufferMessages (500). Oldest entry is dropped
//     until both constraints hold (REQ-BR-009).
//   - Entries older than BufferTTL (24h) are evicted lazily on every Append
//     and Replay. The 24h window is anchored on the cookie lifetime
//     (REQ-BR-002) so a session that resumes within the cookie's validity
//     never observes a gap caused by TTL expiry.
//   - Replay returns every entry whose Sequence > lastSeq, in original
//     order. Sequence gaps within the buffer are impossible: sequences are
//     allocated by outboundDispatcher.nextSequence which is monotonic and
//     keyed by LogicalID (REQ-BR-AMEND-006).
//
// Concurrency: a single sync.Mutex guards the per-key map and each
// queue. Operations are O(1) amortised for Append (slice copy on eviction
// is bounded by MaxBufferMessages) and O(n) for Replay where n is the
// number of replayed entries.

package bridge

import (
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
)

// Buffer limits per spec.md §3.1 item 8 / REQ-BR-009.
const (
	MaxBufferBytes    = 4 * 1024 * 1024 // 4 MiB
	MaxBufferMessages = 500
	BufferTTL         = 24 * time.Hour
)

// bufferEntry pairs an OutboundMessage with its enqueue timestamp so the
// 24h TTL can be enforced without inspecting the message envelope.
type bufferEntry struct {
	msg     OutboundMessage
	addedAt time.Time
	bytes   int
}

// outboundBuffer is a per-LogicalID FIFO of outbound messages used for
// reconnect replay (M4-T2) and offline buffering (M4-T1). Keys are
// LogicalID strings (AMEND-001 M3; previously connIDs). Multiple connIDs
// that share the same (CookieHash, Transport) will share a single
// LogicalID-keyed queue, enabling cross-connection replay.
//
// @MX:ANCHOR
// @MX:REASON Replay correctness invariant: any outbound message handed to
// the buffer must be visible to a subsequent Replay(lastSeq) until either
// the buffer is full and evicts oldest, or 24h pass.
// @MX:NOTE: [AUTO] Buffer key changed from connID to LogicalID in
// SPEC-GOOSE-BRIDGE-001-AMEND-001 v0.1.1 (REQ-BR-AMEND-003). The string
// key type is unchanged; only the semantics of what the key represents
// has shifted. Registry-less callers and fallback paths continue to use
// connID directly.
type outboundBuffer struct {
	clock clockwork.Clock

	mu     sync.Mutex
	queues map[string][]bufferEntry
	bytes  map[string]int // running byte total per session
}

func newOutboundBuffer(clock clockwork.Clock) *outboundBuffer {
	if clock == nil {
		clock = clockwork.NewRealClock()
	}
	return &outboundBuffer{
		clock:  clock,
		queues: make(map[string][]bufferEntry),
		bytes:  make(map[string]int),
	}
}

// Append records msg using msg.SessionID as the buffer key. Since
// AMEND-001 M3, the outbound dispatcher sets msg.SessionID to the
// LogicalID (not the connID) before calling Append, so the buffer queue
// is keyed by LogicalID. Registry-less callers that skip the LogicalID
// lookup use the connID directly as the key (fallback path).
//
// The message is appended to the tail of the per-key queue and contributes
// len(msg.Payload) bytes to the key's running byte total. On entry,
// expired entries are evicted; on exit, oldest-drop eviction is applied
// until both byte and count limits hold.
func (b *outboundBuffer) Append(msg OutboundMessage) {
	now := b.clock.Now()
	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.queues[msg.SessionID]
	q = b.evictExpiredLocked(msg.SessionID, q, now)

	entry := bufferEntry{
		msg:     msg,
		addedAt: now,
		bytes:   len(msg.Payload),
	}
	q = append(q, entry)
	b.bytes[msg.SessionID] += entry.bytes

	// Oldest-drop until both limits are satisfied.
	for len(q) > MaxBufferMessages || b.bytes[msg.SessionID] > MaxBufferBytes {
		head := q[0]
		q = q[1:]
		b.bytes[msg.SessionID] -= head.bytes
	}

	b.queues[msg.SessionID] = q
}

// Replay returns the messages for the given key (LogicalID since
// AMEND-001 M3) with Sequence strictly greater than lastSeq, in enqueue
// order. Entries past the 24h TTL are evicted as a side effect. The
// returned slice is a copy; callers may mutate it freely.
func (b *outboundBuffer) Replay(sessionID string, lastSeq uint64) []OutboundMessage {
	now := b.clock.Now()
	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.evictExpiredLocked(sessionID, b.queues[sessionID], now)
	b.queues[sessionID] = q
	if len(q) == 0 {
		return nil
	}
	out := make([]OutboundMessage, 0, len(q))
	for _, e := range q {
		if e.msg.Sequence > lastSeq {
			out = append(out, e.msg)
		}
	}
	return out
}

// Drop removes all buffered state for the given key (LogicalID since
// AMEND-001 M3). Called on logout eager-drop (REQ-BR-AMEND-007) or on
// session close to release memory.
func (b *outboundBuffer) Drop(sessionID string) {
	b.mu.Lock()
	delete(b.queues, sessionID)
	delete(b.bytes, sessionID)
	b.mu.Unlock()
}

// Len returns the queued message count for the given key (LogicalID since
// AMEND-001 M3). Test-only helper; not part of the production wire path.
func (b *outboundBuffer) Len(sessionID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.queues[sessionID])
}

// Bytes returns the queued byte total for the given key (LogicalID since
// AMEND-001 M3). Test-only helper.
func (b *outboundBuffer) Bytes(sessionID string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.bytes[sessionID]
}

// evictExpiredLocked drops entries older than BufferTTL relative to now and
// returns the trimmed queue. Caller must hold b.mu.
func (b *outboundBuffer) evictExpiredLocked(sessionID string, q []bufferEntry, now time.Time) []bufferEntry {
	cutoff := now.Add(-BufferTTL)
	dropped := 0
	for dropped < len(q) && q[dropped].addedAt.Before(cutoff) {
		b.bytes[sessionID] -= q[dropped].bytes
		dropped++
	}
	if dropped == 0 {
		return q
	}
	return q[dropped:]
}
