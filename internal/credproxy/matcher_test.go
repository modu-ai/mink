package credproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMatchHostPatternExactMatch tests exact host matching.
func TestMatchHostPatternExactMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		host     string
		expected bool
	}{
		{
			name:     "exact match",
			pattern:  "api.openai.com",
			host:     "api.openai.com",
			expected: true,
		},
		{
			name:     "exact match different",
			pattern:  "api.anthropic.com",
			host:     "api.openai.com",
			expected: false,
		},
		{
			name:     "wildcard matches all",
			pattern:  "*",
			host:     "any.host.com",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchHostPattern(tt.pattern, tt.host)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMatchHostPatternWildcard tests wildcard matching.
func TestMatchHostPatternWildcard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		host     string
		expected bool
	}{
		{
			name:     "wildcard matches subdomain",
			pattern:  "*.openai.com",
			host:     "api.openai.com",
			expected: true,
		},
		{
			name:     "wildcard matches multiple subdomains",
			pattern:  "*.openai.com",
			host:     "v2.api.openai.com",
			expected: true, // Our glob matcher uses **-like recursive matching
		},
		{
			name:     "wildcard at end",
			pattern:  "api.*",
			host:     "api.openai.com",
			expected: true, // * matches everything after the dot
		},
		{
			name:     "question mark matches single char",
			pattern:  "api?.openai.com",
			host:     "api1.openai.com",
			expected: true,
		},
		{
			name:     "question mark no match",
			pattern:  "api?.openai.com",
			host:     "api12.openai.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchHostPattern(tt.pattern, tt.host)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestNormalizeHost tests host normalization.
func TestNormalizeHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{
			name:     "lowercase conversion",
			host:     "API.OpenAI.COM",
			expected: "api.openai.com",
		},
		{
			name:     "remove port number",
			host:     "api.openai.com:443",
			expected: "api.openai.com",
		},
		{
			name:     "remove port with custom port",
			host:     "localhost:8080",
			expected: "localhost",
		},
		{
			name:     "no changes needed",
			host:     "api.openai.com",
			expected: "api.openai.com",
		},
		{
			name:     "lowercase and remove port",
			host:     "API.OpenAI.COM:443",
			expected: "api.openai.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeHost(tt.host)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExtractHost tests host extraction from URLs.
func TestExtractHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     string
		expected string
	}{
		{
			name:     "remove https scheme",
			addr:     "https://api.openai.com",
			expected: "api.openai.com",
		},
		{
			name:     "remove http scheme",
			addr:     "http://api.openai.com",
			expected: "api.openai.com",
		},
		{
			name:     "remove port",
			addr:     "api.openai.com:443",
			expected: "api.openai.com",
		},
		{
			name:     "remove path",
			addr:     "api.openai.com/v1/chat",
			expected: "api.openai.com",
		},
		{
			name:     "remove scheme, port, and path",
			addr:     "https://api.openai.com:443/v1/chat",
			expected: "api.openai.com",
		},
		{
			name:     "already just host",
			addr:     "api.openai.com",
			expected: "api.openai.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractHost(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMatchHostPatternEdgeCases tests edge cases for host matching.
func TestMatchHostPatternEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pattern  string
		host     string
		expected bool
	}{
		{
			name:     "empty pattern",
			pattern:  "",
			host:     "api.openai.com",
			expected: false,
		},
		{
			name:     "empty host",
			pattern:  "api.openai.com",
			host:     "",
			expected: false,
		},
		{
			name:     "both empty",
			pattern:  "",
			host:     "",
			expected: true, // Empty pattern matches empty host in our glob implementation
		},
		{
			name:     "pattern with spaces",
			pattern:  "api .openai.com",
			host:     "api.openai.com",
			expected: false,
		},
		{
			name:     "host with spaces",
			pattern:  "api.openai.com",
			host:     "api .openai.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchHostPattern(tt.pattern, tt.host)
			assert.Equal(t, tt.expected, result)
		})
	}
}
