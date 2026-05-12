package compressor

import (
	"github.com/modu-ai/mink/internal/learning/trajectory"
)

// findProtectedIndices returns the set of conversation indices that must not be
// compressed.
//
// Head protection: first occurrence of each distinct role in {system, human, gpt, tool}.
// Tail protection: last tailProtectedTurns entries.
// redacted_thinking protection: any entry whose Value contains a redacted_thinking marker.
//
// REQ-COMPRESSOR-003, REQ-COMPRESSOR-021
func findProtectedIndices(t *trajectory.Trajectory, tailProtectedTurns int) map[int]struct{} {
	protected := make(map[int]struct{})
	n := len(t.Conversations)
	if n == 0 {
		return protected
	}

	// Head: first occurrence of each role
	seenRoles := make(map[trajectory.Role]bool)
	protectedRoles := map[trajectory.Role]bool{
		trajectory.RoleSystem: true,
		trajectory.RoleHuman:  true,
		trajectory.RoleGPT:    true,
		trajectory.RoleTool:   true,
	}
	for i, entry := range t.Conversations {
		if protectedRoles[entry.From] && !seenRoles[entry.From] {
			seenRoles[entry.From] = true
			protected[i] = struct{}{}
		}
	}

	// Tail: last tailProtectedTurns entries
	if tailProtectedTurns > 0 {
		start := n - tailProtectedTurns
		if start < 0 {
			start = 0
		}
		for i := start; i < n; i++ {
			protected[i] = struct{}{}
		}
	}

	// redacted_thinking: protect entries containing redacted_thinking markers
	// REQ-COMPRESSOR-021(a)
	for i, entry := range t.Conversations {
		if hasRedactedThinking(entry.Value) {
			protected[i] = struct{}{}
		}
	}

	return protected
}

// hasRedactedThinking checks whether a conversation entry value contains a
// redacted_thinking placeholder. The placeholder convention is the literal
// string "<redacted_thinking>" inserted by the trajectory collector.
func hasRedactedThinking(value string) bool {
	return containsRedactedThinkingMarker(value)
}

// containsRedactedThinkingMarker performs a simple substring search.
// This is the canonical detection point for redacted_thinking content.
func containsRedactedThinkingMarker(s string) bool {
	return len(s) >= len(redactedThinkingMarker) && findSubstring(s, redactedThinkingMarker)
}

// redactedThinkingMarker is the placeholder string injected by the trajectory collector
// when a redacted_thinking content block is encountered.
const redactedThinkingMarker = "<redacted_thinking>"

// findSubstring is a simple O(n*m) search used to avoid importing strings in this file.
// For production scale this is fine: entries are typically short (< 10 KB).
func findSubstring(haystack, needle string) bool {
	h, n := len(haystack), len(needle)
	if n > h {
		return false
	}
	for i := 0; i <= h-n; i++ {
		if haystack[i:i+n] == needle {
			return true
		}
	}
	return false
}

// findCompressibleRegion returns the [compressStart, compressEnd) index range of
// the longest contiguous unprotected segment between head and tail regions.
//
// REQ-COMPRESSOR-011: if compressStart >= compressEnd, no compressible region exists.
func findCompressibleRegion(protected map[int]struct{}, totalTurns int) (compressStart, compressEnd int) {
	// Walk forward from index 0 to find where the head-protected region ends.
	// The compressible region starts at the first non-protected index after the head.
	compressStart = 0
	for compressStart < totalTurns {
		if _, ok := protected[compressStart]; !ok {
			break
		}
		compressStart++
	}

	// Walk backward from the last index to find where the tail-protected region starts.
	compressEnd = totalTurns
	for compressEnd > compressStart {
		if _, ok := protected[compressEnd-1]; !ok {
			break
		}
		compressEnd--
	}

	return compressStart, compressEnd
}
