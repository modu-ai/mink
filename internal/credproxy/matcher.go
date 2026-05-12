package credproxy

import (
	"strings"

	"github.com/modu-ai/mink/internal/fsaccess"
)

// matchHostPattern tests whether a host matches a host pattern.
// This implements REQ-CREDPROXY-004: Scoped credential binding.
//
// @MX:ANCHOR: [AUTO] Host pattern matching function
// @MX:REASON: Core security enforcement for scoped credential binding, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-004, AC-CREDPROXY-03
func matchHostPattern(pattern, host string) bool {
	// Fast path: exact match
	if pattern == host {
		return true
	}

	// Fast path: wildcard pattern matches any host
	if pattern == "*" {
		return true
	}

	// Use fsaccess.GlobMatch for pattern matching
	// This supports *, ?, and ** wildcards with path-aware matching
	// For host patterns, we treat the host as a single path component
	return fsaccess.GlobMatch(pattern, host)
}

// normalizeHost normalizes a hostname for pattern matching.
// This removes port numbers and converts to lowercase.
func normalizeHost(host string) string {
	// Remove port number if present
	if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	// Convert to lowercase for case-insensitive matching
	return strings.ToLower(host)
}

// extractHost extracts the hostname from a URL or address.
// This handles URLs with schemes (https://api.openai.com) and
// addresses with ports (api.openai.com:443).
func extractHost(addr string) string {
	// Remove scheme if present
	if strings.HasPrefix(addr, "http://") {
		addr = strings.TrimPrefix(addr, "http://")
	} else if strings.HasPrefix(addr, "https://") {
		addr = strings.TrimPrefix(addr, "https://")
	}

	// Remove path if present
	if slashIdx := strings.Index(addr, "/"); slashIdx != -1 {
		addr = addr[:slashIdx]
	}

	// Remove port if present
	if colonIdx := strings.Index(addr, ":"); colonIdx != -1 {
		addr = addr[:colonIdx]
	}

	return addr
}
