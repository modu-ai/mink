// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-002
// AC: AC-BR-AMEND-002
// M2-T1 — unit + concurrency tests for Registry.LogicalID.

package bridge

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// newTestSessionWithLogicalID returns a WebUISession with the given LogicalID
// populated in addition to the standard test fields.
func newTestSessionWithLogicalID(id string, transport Transport, logicalID string) WebUISession {
	return WebUISession{
		ID:           id,
		CookieHash:   []byte("hash-" + id),
		CSRFHash:     []byte("csrf-" + id),
		Transport:    transport,
		OpenedAt:     time.Now(),
		LastActivity: time.Now(),
		State:        SessionStateOpen,
		LogicalID:    logicalID,
	}
}

// TestRegistry_LogicalID_Hit verifies that a session registered with a
// non-empty LogicalID is returned as (value, true).
// Covers AC-BR-AMEND-002 (happy path).
func TestRegistry_LogicalID_Hit(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	s := newTestSessionWithLogicalID("sid-hit", TransportWebSocket, "L1")
	if err := r.Add(s); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, ok := r.LogicalID("sid-hit")
	if !ok {
		t.Fatalf("LogicalID(sid-hit) ok = false, want true")
	}
	if got != "L1" {
		t.Fatalf("LogicalID(sid-hit) = %q, want %q", got, "L1")
	}
}

// TestRegistry_LogicalID_Miss verifies that looking up an unregistered connID
// returns ("", false).
// Covers AC-BR-AMEND-002 (miss path).
func TestRegistry_LogicalID_Miss(t *testing.T) {
	t.Parallel()

	r := NewRegistry()

	got, ok := r.LogicalID("sid-zzz")
	if ok {
		t.Fatalf("LogicalID(unregistered) ok = true, want false")
	}
	if got != "" {
		t.Fatalf("LogicalID(unregistered) = %q, want empty", got)
	}
}

// TestRegistry_LogicalID_EmptyValue verifies that a session registered with
// an empty LogicalID is treated as a miss — ("", false).
//
// This is the migration/pre-fill scenario: a session was added before the
// LogicalID field was introduced and its value is the zero string. Per
// AC-BR-AMEND-002, empty LogicalID is semantically equivalent to no LogicalID.
// Covers AC-BR-AMEND-002 (empty-value semantic miss).
func TestRegistry_LogicalID_EmptyValue(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	// Add session with empty LogicalID (zero-value field).
	s := newTestSession("sid-empty", TransportWebSocket)
	// s.LogicalID is already "" (zero value) — do not set it.
	if err := r.Add(s); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, ok := r.LogicalID("sid-empty")
	if ok {
		t.Fatalf("LogicalID(empty LogicalID session) ok = true, want false")
	}
	if got != "" {
		t.Fatalf("LogicalID(empty LogicalID session) = %q, want empty", got)
	}
}

// TestRegistry_LogicalID_Concurrent verifies that Registry.LogicalID is safe
// under concurrent reads and writes (no data race, correct mutex behaviour).
//
// 100 goroutines run in two groups:
//   - First 50 register sessions with unique connIDs and LogicalIDs.
//   - Last 50 repeatedly look up connIDs (both registered and unregistered).
//
// The test passes under `go test -race -count=10`.
// Covers AC-BR-AMEND-002 (concurrent-safety / mutex correctness).
func TestRegistry_LogicalID_Concurrent(t *testing.T) {
	t.Parallel()

	r := NewRegistry()
	const total = 100

	var wg sync.WaitGroup
	wg.Add(total)

	// Writers: goroutines 0..49 each register one session.
	for i := range total / 2 {
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("conc-sid-%d", n)
			logID := fmt.Sprintf("conc-L%d", n)
			s := newTestSessionWithLogicalID(id, TransportWebSocket, logID)
			_ = r.Add(s)
		}(i)
	}

	// Readers: goroutines 50..99 look up IDs (some will miss, some will hit).
	for i := range total / 2 {
		go func(n int) {
			defer wg.Done()
			id := fmt.Sprintf("conc-sid-%d", n)
			_, _ = r.LogicalID(id)
		}(i)
	}

	wg.Wait()
}

// TestRegistry_TwoConnectionsSameCookie_ShareLogicalID verifies the end-to-end
// invariant: two WebUISessions derived from the same (cookieHash, transport)
// via Authenticator.DeriveLogicalID share the same LogicalID value, and
// Registry.LogicalID returns identical values for both connIDs.
//
// This is the integration validation of REQ-BR-AMEND-001 × REQ-BR-AMEND-002:
// the deterministic HMAC derivation + registry lookup combine correctly.
// Covers AC-BR-AMEND-002 (cross-connection same-LogicalID invariant).
func TestRegistry_TwoConnectionsSameCookie_ShareLogicalID(t *testing.T) {
	t.Parallel()

	auth := newTestAuthenticator(t)
	cookieHash := []byte("same-cookie-hash-for-both-tabs-x") // 32 bytes

	// Both connections use the same cookie and the same transport (WebSocket).
	logicalID := auth.DeriveLogicalID(cookieHash, TransportWebSocket)
	if logicalID == "" {
		t.Fatal("DeriveLogicalID returned empty string for non-empty cookieHash")
	}

	r := NewRegistry()

	// Tab A: first connection.
	sessA := WebUISession{
		ID:           "sid-aaa",
		CookieHash:   cookieHash,
		Transport:    TransportWebSocket,
		OpenedAt:     time.Now(),
		LastActivity: time.Now(),
		State:        SessionStateOpen,
		LogicalID:    logicalID,
	}
	if err := r.Add(sessA); err != nil {
		t.Fatalf("Add(sid-aaa): %v", err)
	}

	// Tab B: second connection with the same cookie, same transport.
	logicalIDB := auth.DeriveLogicalID(cookieHash, TransportWebSocket)
	if logicalIDB != logicalID {
		t.Fatalf("DeriveLogicalID should be deterministic: got %q and %q", logicalID, logicalIDB)
	}

	sessB := WebUISession{
		ID:           "sid-bbb",
		CookieHash:   cookieHash,
		Transport:    TransportWebSocket,
		OpenedAt:     time.Now(),
		LastActivity: time.Now(),
		State:        SessionStateOpen,
		LogicalID:    logicalIDB,
	}
	if err := r.Add(sessB); err != nil {
		t.Fatalf("Add(sid-bbb): %v", err)
	}

	// Both registry lookups must return the same LogicalID.
	gotA, okA := r.LogicalID("sid-aaa")
	if !okA {
		t.Fatal("LogicalID(sid-aaa) ok = false, want true")
	}
	gotB, okB := r.LogicalID("sid-bbb")
	if !okB {
		t.Fatal("LogicalID(sid-bbb) ok = false, want true")
	}

	if gotA != gotB {
		t.Fatalf("LogicalID mismatch: sid-aaa=%q sid-bbb=%q — same cookie+transport must share LogicalID", gotA, gotB)
	}
	if gotA != logicalID {
		t.Fatalf("LogicalID(sid-aaa) = %q, want %q", gotA, logicalID)
	}
}
