// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-001
// AC: AC-BR-AMEND-001
// M1-T1 — unit tests for DeriveLogicalID (RED phase).

package bridge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"testing"
)

// newTestAuthenticator returns an Authenticator with a fixed 32-byte secret
// for deterministic test scenarios.
func newTestAuthenticator(t *testing.T) *Authenticator {
	t.Helper()
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1) // deterministic non-zero bytes
	}
	a, err := NewAuthenticator(AuthConfig{HMACSecret: secret})
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}
	return a
}

// TestDeriveLogicalID_Deterministic verifies that the same (cookieHash,
// transport) pair produces identical output across 100 invocations.
// Covers AC-BR-AMEND-001 (determinism criterion).
func TestDeriveLogicalID_Deterministic(t *testing.T) {
	t.Parallel()
	a := newTestAuthenticator(t)
	cookieHash := []byte("abcdefghijklmnopqrstuvwxyz012345") // 32 bytes

	first := a.DeriveLogicalID(cookieHash, TransportWebSocket)
	if first == "" {
		t.Fatal("DeriveLogicalID returned empty string for non-empty cookieHash")
	}

	for i := 0; i < 100; i++ {
		got := a.DeriveLogicalID(cookieHash, TransportWebSocket)
		if got != first {
			t.Fatalf("iteration %d: got %q, want %q", i, got, first)
		}
	}
}

// TestDeriveLogicalID_TransportSeparated verifies that the same cookieHash
// but different transports produce different LogicalIDs.
// Covers AC-BR-AMEND-001 (transport separation criterion).
func TestDeriveLogicalID_TransportSeparated(t *testing.T) {
	t.Parallel()
	a := newTestAuthenticator(t)
	cookieHash := []byte("abcdefghijklmnopqrstuvwxyz012345")

	ws := a.DeriveLogicalID(cookieHash, TransportWebSocket)
	sse := a.DeriveLogicalID(cookieHash, TransportSSE)

	if ws == "" || sse == "" {
		t.Fatal("DeriveLogicalID returned empty string")
	}
	if ws == sse {
		t.Fatalf("expected different LogicalIDs for WS and SSE, both got %q", ws)
	}
}

// TestDeriveLogicalID_CookieHashSeparated verifies that cookieHashes
// differing by a single byte produce different LogicalIDs.
// Covers AC-BR-AMEND-001 (cookieHash sensitivity criterion).
func TestDeriveLogicalID_CookieHashSeparated(t *testing.T) {
	t.Parallel()
	a := newTestAuthenticator(t)

	hash1 := []byte("abcdefghijklmnopqrstuvwxyz012345")
	hash2 := make([]byte, len(hash1))
	copy(hash2, hash1)
	hash2[15] ^= 0x01 // flip a single bit in byte 15

	id1 := a.DeriveLogicalID(hash1, TransportWebSocket)
	id2 := a.DeriveLogicalID(hash2, TransportWebSocket)

	if id1 == id2 {
		t.Fatalf("expected different LogicalIDs for different cookieHashes, both got %q", id1)
	}
}

// TestDeriveLogicalID_EmptyCookieHashRejected verifies that an empty
// cookieHash returns an empty string, signalling "no LogicalID yet".
// Covers AC-BR-AMEND-001 (edge-case rejection criterion).
func TestDeriveLogicalID_EmptyCookieHashRejected(t *testing.T) {
	t.Parallel()
	a := newTestAuthenticator(t)

	got := a.DeriveLogicalID(nil, TransportWebSocket)
	if got != "" {
		t.Fatalf("expected empty string for nil cookieHash, got %q", got)
	}

	got = a.DeriveLogicalID([]byte{}, TransportWebSocket)
	if got != "" {
		t.Fatalf("expected empty string for empty cookieHash, got %q", got)
	}
}

// TestDeriveLogicalID_DomainPrefixPresent verifies that the domain-separator
// prefix "bridge-logical-id-v1\x00" is included in the HMAC input.
//
// Strategy: re-compute HMAC manually with the expected prefix and verify it
// matches DeriveLogicalID output. Then re-compute without the prefix and
// verify the result is different — proving the prefix is load-bearing.
//
// Covers AC-BR-AMEND-001 (NIST SP 800-108 domain-separator criterion, D5 fix).
func TestDeriveLogicalID_DomainPrefixPresent(t *testing.T) {
	t.Parallel()

	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	a, err := NewAuthenticator(AuthConfig{HMACSecret: secret})
	if err != nil {
		t.Fatalf("NewAuthenticator: %v", err)
	}

	cookieHash := []byte("abcdefghijklmnopqrstuvwxyz012345")
	transport := TransportWebSocket

	// Compute reference value using the full DeriveLogicalID path.
	derivedID := a.DeriveLogicalID(cookieHash, transport)
	if derivedID == "" {
		t.Fatal("DeriveLogicalID returned empty string")
	}

	// Re-compute manually WITH the domain prefix — must match derivedID.
	withPrefix := manualDeriveLogicalID(secret, []byte("bridge-logical-id-v1\x00"), cookieHash, string(transport))
	if withPrefix != derivedID {
		t.Fatalf(
			"manual computation WITH prefix does not match DeriveLogicalID:\n  got  %q\n  want %q",
			withPrefix, derivedID,
		)
	}

	// Re-compute manually WITHOUT the domain prefix — must NOT match derivedID,
	// proving the prefix is actually incorporated in the HMAC input.
	withoutPrefix := manualDeriveLogicalID(secret, nil, cookieHash, string(transport))
	if withoutPrefix == derivedID {
		t.Fatalf(
			"manual computation WITHOUT prefix equals DeriveLogicalID — prefix is not load-bearing: %q",
			derivedID,
		)
	}
}

// manualDeriveLogicalID is a helper used only in TestDeriveLogicalID_DomainPrefixPresent.
// It re-implements the HMAC computation so the test is independent of the
// production implementation while still verifying the spec invariant.
// Returns the first 32 characters of the RawURL-encoded digest, matching the
// production truncation in DeriveLogicalID.
func manualDeriveLogicalID(secret, prefix, cookieHash []byte, transport string) string {
	h := hmac.New(sha256.New, secret)
	if len(prefix) > 0 {
		h.Write(prefix)
	}
	// length-prefix for cookieHash (uvarint encoding)
	var lenBuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(lenBuf[:], uint64(len(cookieHash)))
	h.Write(lenBuf[:n])
	h.Write(cookieHash)
	h.Write([]byte(transport))
	encoded := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	return encoded[:32]
}
