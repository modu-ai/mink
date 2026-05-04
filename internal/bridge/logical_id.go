// SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001
// REQ: REQ-BR-AMEND-001
// AC: AC-BR-AMEND-001
// M1-T1 — DeriveLogicalID: stable per-(CookieHash, Transport) session identifier.

package bridge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
)

// logicalIDDomainPrefix is the NIST SP 800-108 domain-separator prefix
// prepended to the HMAC input so that LogicalID derivation shares the cookie
// HMAC secret without creating a key-reuse oracle.
//
// The 21-byte prefix (20 ASCII chars + NUL terminator) is byte-wise disjoint
// from the cookie-signing input space (24-byte fixed-length payload at
// auth.go:102-104), making cross-purpose HMAC collisions structurally
// impossible.
//
// @MX:NOTE: [AUTO] Domain prefix version tag — bump to "bridge-logical-id-v2\x00"
// if the HMAC input format changes (e.g., new fields appended).
// @MX:SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001 REQ-BR-AMEND-001
const logicalIDDomainPrefix = "bridge-logical-id-v1\x00"

// DeriveLogicalID computes a stable, transport-scoped identifier for the
// session identified by (cookieHash, transport).
//
// Algorithm (NIST SP 800-108 key-separation pattern):
//
//	HMAC-SHA256(secret,
//	    "bridge-logical-id-v1\x00"          // 21-byte domain prefix
//	    || uvarint(len(cookieHash))          // length-prefix (boundary attack defense)
//	    || cookieHash                        // variable-length cookie reference
//	    || string(transport))               // "websocket" or "sse"
//
// The output is the first 32 characters of the base64 RawURL encoding of the
// full 32-byte HMAC digest (~192 bits of collision resistance; sufficient for
// the expected cookie population size — see research.md §3.2).
//
// Security properties:
//   - Pre-image resistant: LogicalID leakage does not reveal cookieHash.
//   - Domain-separated: shares the cookie HMAC secret without oracle exposure.
//   - Transport-scoped: WebSocket and SSE produce distinct LogicalIDs even for
//     the same cookie, preventing cross-transport sequence ambiguity.
//   - Length-prefix safe: uvarint(len(cookieHash)) prevents boundary attacks
//     when cookieHash length could vary in future hash-function migrations.
//
// Edge cases:
//   - Returns "" when cookieHash is nil or empty. This signals "session has no
//     LogicalID yet" — callers (Registry.LogicalID) return ("", false) for
//     empty LogicalID values. The dispatcher falls back to connID-keyed
//     buffering in this scenario (REQ-BR-AMEND-003 fallback branch).
//
// @MX:ANCHOR: [AUTO] LogicalID derivation invariant — two sessions sharing the
// same (CookieHash, Transport) MUST produce identical LogicalID values; any
// change to either input MUST produce a different value.
// @MX:REASON: REQ-BR-AMEND-001: determinism + NIST SP 800-108 domain separation.
// Any alteration to the HMAC input format (prefix, encoding, field order)
// breaks cross-connection replay and must be coordinated with a spec amendment.
// @MX:SPEC: SPEC-GOOSE-BRIDGE-001-AMEND-001 REQ-BR-AMEND-001 AC-BR-AMEND-001
func (a *Authenticator) DeriveLogicalID(cookieHash []byte, transport Transport) string {
	if len(cookieHash) == 0 {
		return ""
	}

	h := hmac.New(sha256.New, a.secret)

	// 1. Domain-separator prefix (NIST SP 800-108 key separation).
	h.Write([]byte(logicalIDDomainPrefix))

	// 2. Length-prefix for cookieHash using unsigned varint encoding.
	//    Prevents boundary attacks when len(cookieHash) varies.
	var lenBuf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(lenBuf[:], uint64(len(cookieHash)))
	h.Write(lenBuf[:n])

	// 3. Cookie hash bytes.
	h.Write(cookieHash)

	// 4. Transport string ("websocket" or "sse"). These two values are not
	//    prefix-free of each other relative to the preceding fields, but the
	//    length-prefix on cookieHash makes the concatenation unambiguous.
	h.Write([]byte(transport))

	digest := h.Sum(nil)
	// Encode the full 32-byte digest and return the first 32 characters
	// (~192 bits of effective entropy after base64url alphabet reduction).
	encoded := base64.RawURLEncoding.EncodeToString(digest)
	return encoded[:32]
}
