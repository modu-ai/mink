package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ComputeEventHash computes SHA256 hash of an audit event for chain integrity.
// The hash is computed over a deterministic string representation of event fields.
//
// @MX:ANCHOR: [AUTO] Core hash computation for audit event integrity
// @MX:REASON: Used by FileWriter for hash chaining and VerifyChain for validation, fan_in >= 2
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 integrity verification requirement
func ComputeEventHash(event AuditEvent) string {
	var buf strings.Builder
	buf.WriteString(event.Timestamp.String())
	buf.WriteByte(0x1F) // unit separator
	buf.WriteString(string(event.Type))
	buf.WriteByte(0x1F)
	buf.WriteString(string(event.Severity))
	buf.WriteByte(0x1F)
	buf.WriteString(event.Message)
	buf.WriteByte(0x1F)

	// Sort metadata keys for deterministic ordering
	keys := make([]string, 0, len(event.Metadata))
	for k := range event.Metadata {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteByte(0x1E) // record separator
		buf.WriteString(event.Metadata[k])
		buf.WriteByte(0x1F)
	}

	buf.WriteString(event.PrevHash)

	hash := sha256.Sum256([]byte(buf.String()))
	return hex.EncodeToString(hash[:])
}

// VerifyChain verifies the integrity of a slice of audit events.
// Returns the index of the first broken link, or -1 if chain is intact.
// Events with empty PrevHash are treated as chain anchors (no verification needed).
//
// @MX:ANCHOR: [AUTO] Chain integrity verification for audit logs
// @MX:REASON: Core validation function used by audit verification tools, fan_in >= 2
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-005 integrity chain verification
func VerifyChain(events []AuditEvent) (int, error) {
	for i := 1; i < len(events); i++ {
		// Skip verification for anchor events (empty PrevHash)
		if events[i].PrevHash == "" {
			continue
		}
		// Verify that PrevHash matches hash of previous event
		expectedHash := ComputeEventHash(events[i-1])
		if events[i].PrevHash != expectedHash {
			return i, nil
		}
	}
	return -1, nil
}
