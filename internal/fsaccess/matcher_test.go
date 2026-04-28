// Package fsaccess provides filesystem access control with policy-based security.
// SPEC-GOOSE-FS-ACCESS-001
package fsaccess

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGlobMatch_WildcardSingle tests single-character wildcard (?)
func TestGlobMatch_WildcardSingle(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "question mark matches single character",
			pattern:  "file?.txt",
			path:     "file1.txt",
			expected: true,
		},
		{
			name:     "question mark matches single char at start",
			pattern:  "?ile.txt",
			path:     "file.txt",
			expected: true,
		},
		{
			name:     "question mark does not match multiple characters",
			pattern:  "file?.txt",
			path:     "file12.txt",
			expected: false,
		},
		{
			name:     "question mark does not match dot in leading dotfile",
			pattern:  ".env?",
			path:     ".env",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GlobMatch(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGlobMatch_WildcardMulti tests multi-character wildcard (*)
func TestGlobMatch_WildcardMulti(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "asterisk matches zero characters",
			pattern:  "*.txt",
			path:     ".txt",
			expected: true,
		},
		{
			name:     "asterisk matches multiple characters",
			pattern:  "*.txt",
			path:     "file.txt",
			expected: true,
		},
		{
			name:     "asterisk matches in middle",
			pattern:  "file*.txt",
			path:     "file123.txt",
			expected: true,
		},
		{
			name:     "asterisk does not match path separator",
			pattern:  "*.txt",
			path:     "dir/file.txt",
			expected: false,
		},
		{
			name:     "double asterisk matches path separator",
			pattern:  "**/*.txt",
			path:     "dir/file.txt",
			expected: true,
		},
		{
			name:     "asterisk matches extension",
			pattern:  "file.*",
			path:     "file.txt",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GlobMatch(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGlobMatch_DoubleStarRecursive tests recursive double-star (**) matching
func TestGlobMatch_DoubleStarRecursive(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "double star matches zero directories",
			pattern:  "**/file.txt",
			path:     "file.txt",
			expected: true,
		},
		{
			name:     "double star matches single directory",
			pattern:  "**/file.txt",
			path:     "dir/file.txt",
			expected: true,
		},
		{
			name:     "double star matches multiple directories",
			pattern:  "**/file.txt",
			path:     "a/b/c/file.txt",
			expected: true,
		},
		{
			name:     "double star matches at start",
			pattern:  "**/file.txt",
			path:     "deep/nested/path/file.txt",
			expected: true,
		},
		{
			name:     "double star matches in middle",
			pattern:  "/home/**/*.txt",
			path:     "/home/user/docs/file.txt",
			expected: true,
		},
		{
			name:     "double star matches all files recursively",
			pattern:  "./.goose/**",
			path:     "./.goose/deep/nested/file.txt",
			expected: true,
		},
		{
			name:     "double star with double extension",
			pattern:  "drafts/**/*.md",
			path:     "drafts/a/b/c/file.md",
			expected: true,
		},
		{
			name:     "double star matches specific file deep",
			pattern:  "/**/.env",
			path:     "/deep/path/.env",
			expected: true,
		},
		{
			name:     "double star does not match different extension",
			pattern:  "**/file.txt",
			path:     "dir/file.md",
			expected: false,
		},
		{
			name:     "double star matches with leading dot",
			pattern:  "**/.env",
			path:     ".env",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GlobMatch(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGlobMatch_AbsolutePaths tests absolute path patterns
func TestGlobMatch_AbsolutePaths(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "absolute pattern matches absolute path",
			pattern:  "/etc/**",
			path:     "/etc/hosts",
			expected: true,
		},
		{
			name:     "absolute pattern does not match relative path",
			pattern:  "/etc/**",
			path:     "etc/hosts",
			expected: false,
		},
		{
			name:     "absolute pattern with specific file",
			pattern:  "/var/log/**",
			path:     "/var/log/system.log",
			expected: true,
		},
		{
			name:     "absolute pattern with double star",
			pattern:  "/**/.env",
			path:     "/home/user/project/.env",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GlobMatch(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGlobMatch_RelativePaths tests relative path patterns
func TestGlobMatch_RelativePaths(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "relative pattern matches relative path",
			pattern:  "./.goose/**",
			path:     "./.goose/file.txt",
			expected: true,
		},
		{
			name:     "relative pattern without dot",
			pattern:  ".goose/**",
			path:     ".goose/file.txt",
			expected: true,
		},
		{
			name:     "double star matches nested directories",
			pattern:  "./drafts/**/*.md",
			path:     "./drafts/nested/file.md",
			expected: true,
		},
		{
			name:     "match all in current directory",
			pattern:  "./**",
			path:     "./any/file/here.txt",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GlobMatch(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGlobMatch_TildeExpansion tests tilde home directory patterns
func TestGlobMatch_TildeExpansion(t *testing.T) {
	// Note: Tilde expansion should be handled by the caller
	// GlobMatch treats tilde as a literal character
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "tilde in pattern matches tilde in path",
			pattern:  "~/.ssh/**",
			path:     "~/.ssh/config",
			expected: true,
		},
		{
			name:     "tilde does not match expanded path",
			pattern:  "~/.ssh/**",
			path:     "/home/user/.ssh/config",
			expected: false,
		},
		{
			name:     "tilde with double star",
			pattern:  "~/.config/goose/**",
			path:     "~/.config/goose/settings.json",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GlobMatch(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGlobMatch_EdgeCases tests edge cases
func TestGlobMatch_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "empty pattern matches empty path",
			pattern:  "",
			path:     "",
			expected: true,
		},
		{
			name:     "empty pattern does not match non-empty path",
			pattern:  "",
			path:     "file.txt",
			expected: false,
		},
		{
			name:     "exact match without wildcards",
			pattern:  "file.txt",
			path:     "file.txt",
			expected: true,
		},
		{
			name:     "exact mismatch",
			pattern:  "file.txt",
			path:     "other.txt",
			expected: false,
		},
		{
			name:     "pattern with special regex chars",
			pattern:  "file[123].txt",
			path:     "file[123].txt",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GlobMatch(tt.pattern, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}
