// Package fsaccess provides filesystem access control with policy-based security.
// SPEC-GOOSE-FS-ACCESS-001
package fsaccess

import (
	"path/filepath"
	"strings"
)

// GlobMatch tests whether a path matches a glob pattern.
// It supports the following special patterns:
//
//   - matches any sequence of characters except path separator
//     ?    matches any single character
//     **   matches any sequence of directories (zero or more)
//
// The function does NOT perform tilde (~) expansion - that should be
// handled by the caller before matching.
//
// REQ-FSACCESS-002: Glob pattern matching precision
// AC-02: Support **, *, ? wildcards with path-aware matching
//
// @MX:ANCHOR: [AUTO] Core glob matching function
// @MX:REASON: Used by DecisionEngine for all path matching, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-002, AC-02
func GlobMatch(pattern, path string) bool {
	// Fast path: exact match
	if pattern == path {
		return true
	}

	// Fast path: empty pattern matches empty path only
	if pattern == "" {
		return path == ""
	}

	// Handle ** recursive matching
	if strings.Contains(pattern, "**") {
		return matchWithDoubleStar(pattern, path)
	}

	// Use standard filepath.Match for simple patterns
	// This handles * and ? but not **
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		// Invalid pattern - treat as no match
		return false
	}
	return matched
}

// matchWithDoubleStar handles patterns containing ** for recursive directory matching.
// The ** pattern matches zero or more path components.
//
// Examples:
//
//	"**/file.txt" matches "file.txt", "dir/file.txt", "a/b/c/file.txt"
//	"/home/**/*.txt" matches "/home/file.txt", "/home/dir/file.txt"
//	"/**/.env" matches "/.env", "/dir/.env", "/a/b/c/.env"
func matchWithDoubleStar(pattern, targetPath string) bool {
	// Split pattern and path by path separator
	patternParts := splitPath(pattern)
	pathParts := splitPath(targetPath)

	// Match each part
	return matchParts(patternParts, pathParts, 0, 0)
}

// matchParts recursively matches pattern parts against path parts.
// It handles the ** wildcard which can match zero or more path components.
func matchParts(patternParts, pathParts []string, pIdx, pathIdx int) bool {
	// If we've consumed all pattern parts, check if we've also consumed all path parts
	if pIdx >= len(patternParts) {
		return pathIdx >= len(pathParts)
	}

	// If we've consumed all path parts but still have pattern parts,
	// the remaining pattern parts must all be "**" (which can match zero parts)
	if pathIdx >= len(pathParts) {
		// Check if remaining pattern parts are all "**"
		for i := pIdx; i < len(patternParts); i++ {
			if patternParts[i] != "**" {
				return false
			}
		}
		return true
	}

	patternPart := patternParts[pIdx]

	// Handle ** wildcard - matches zero or more path components
	if patternPart == "**" {
		// Try matching zero components (skip the **)
		if matchParts(patternParts, pathParts, pIdx+1, pathIdx) {
			return true
		}

		// Try matching one or more components
		for i := pathIdx; i < len(pathParts); i++ {
			if matchParts(patternParts, pathParts, pIdx+1, i+1) {
				return true
			}
		}
		return false
	}

	// Regular pattern part - must match exactly
	// Check if the pattern part contains * or ? wildcards
	if strings.ContainsAny(patternPart, "*?") {
		// Use filepath.Match for this part
		matched, err := filepath.Match(patternPart, pathParts[pathIdx])
		if err != nil || !matched {
			return false
		}
	} else {
		// Exact match required
		if patternPart != pathParts[pathIdx] {
			return false
		}
	}

	// Move to next part
	return matchParts(patternParts, pathParts, pIdx+1, pathIdx+1)
}

// splitPath splits a path into components, handling both Unix and Windows separators.
// Empty components (from leading/trailing/multiple separators) are preserved
// for proper pattern matching.
func splitPath(p string) []string {
	// Handle empty string
	if p == "" {
		return []string{""}
	}

	// Split on both Unix and Windows path separators
	var parts []string
	current := ""

	for _, ch := range p {
		if ch == '/' || ch == '\\' {
			// Always add a part when we encounter a separator
			// This preserves empty parts for leading/trailing/multiple separators
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}

	// Add the last part
	parts = append(parts, current)

	return parts
}
