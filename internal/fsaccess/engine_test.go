// Package fsaccess provides filesystem access control with policy-based security.
// SPEC-GOOSE-FS-ACCESS-001
package fsaccess

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDecisionEngine_BlockedAlways tests that blocked_always paths are denied unconditionally
func TestDecisionEngine_BlockedAlways(t *testing.T) {
	// Arrange: Create policy with blocked paths
	policy := &SecurityPolicy{
		WritePaths:    []string{"./**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{"~/.ssh/**", "/etc/**", "/**/.env"},
	}

	engine := NewDecisionEngine(policy)

	// Act & Assert: Test various blocked paths
	tests := []struct {
		path      string
		operation Operation
		expected  Decision
	}{
		{
			path:      "~/.ssh/config",
			operation: OperationRead,
			expected:  DecisionDeny,
		},
		{
			path:      "~/.ssh/id_rsa",
			operation: OperationWrite,
			expected:  DecisionDeny,
		},
		{
			path:      "/etc/hosts",
			operation: OperationRead,
			expected:  DecisionDeny,
		},
		{
			path:      "/etc/passwd",
			operation: OperationWrite,
			expected:  DecisionDeny,
		},
		{
			path:      "/project/.env",
			operation: OperationRead,
			expected:  DecisionDeny,
		},
		{
			path:      "/deep/path/to/.env",
			operation: OperationWrite,
			expected:  DecisionDeny,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := engine.CheckAccess(tt.path, tt.operation)
			assert.Equal(t, tt.expected, result.Decision, "decision should be Deny for blocked path")
			assert.Contains(t, result.Reason, "blocked_always", "reason should mention blocked_always")
			assert.NotEmpty(t, result.Policy, "policy should indicate which rule matched")
		})
	}
}

// TestDecisionEngine_WritePaths tests write access control
func TestDecisionEngine_WritePaths(t *testing.T) {
	// Arrange: Create policy with write paths
	policy := &SecurityPolicy{
		WritePaths:    []string{"./.goose/**", "./drafts/**/*.md"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{},
	}

	engine := NewDecisionEngine(policy)

	// Act & Assert: Test write paths
	tests := []struct {
		name      string
		path      string
		operation Operation
		expected  Decision
	}{
		{
			name:      "allowed write in .goose directory",
			path:      "./.goose/config.yaml",
			operation: OperationWrite,
			expected:  DecisionAllow,
		},
		{
			name:      "allowed write in drafts with .md extension",
			path:      "./drafts/chapter1.md",
			operation: OperationWrite,
			expected:  DecisionAllow,
		},
		{
			name:      "allowed write in nested drafts directory",
			path:      "./drafts/2024/01/post.md",
			operation: OperationWrite,
			expected:  DecisionAllow,
		},
		{
			name:      "denied write outside allowed paths",
			path:      "./other/file.txt",
			operation: OperationWrite,
			expected:  DecisionAsk,
		},
		{
			name:      "denied write wrong extension in drafts",
			path:      "./drafts/image.png",
			operation: OperationWrite,
			expected:  DecisionAsk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.CheckAccess(tt.path, tt.operation)
			assert.Equal(t, tt.expected, result.Decision)
			if tt.expected == DecisionAllow {
				assert.Contains(t, result.Reason, "write_paths")
			}
		})
	}
}

// TestDecisionEngine_ReadPaths tests read access control
func TestDecisionEngine_ReadPaths(t *testing.T) {
	// Arrange: Create policy with read paths
	policy := &SecurityPolicy{
		WritePaths:    []string{"./.goose/**"},
		ReadPaths:     []string{"./**", "~/.config/goose/**"},
		BlockedAlways: []string{},
	}

	engine := NewDecisionEngine(policy)

	// Act & Assert: Test read paths
	tests := []struct {
		name      string
		path      string
		operation Operation
		expected  Decision
	}{
		{
			name:      "allowed read in current directory",
			path:      "./file.txt",
			operation: OperationRead,
			expected:  DecisionAllow,
		},
		{
			name:      "allowed read in nested directory",
			path:      "./deep/nested/file.txt",
			operation: OperationRead,
			expected:  DecisionAllow,
		},
		{
			name:      "allowed read in config directory",
			path:      "~/.config/goose/settings.json",
			operation: OperationRead,
			expected:  DecisionAllow,
		},
		{
			name:      "denied read outside allowed paths",
			path:      "/etc/passwd",
			operation: OperationRead,
			expected:  DecisionAsk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.CheckAccess(tt.path, tt.operation)
			assert.Equal(t, tt.expected, result.Decision)
			if tt.expected == DecisionAllow {
				assert.Contains(t, result.Reason, "read_paths")
			}
		})
	}
}

// TestDecisionEngine_CreateOperation tests file creation operations
func TestDecisionEngine_CreateOperation(t *testing.T) {
	// Arrange: Create policy
	policy := &SecurityPolicy{
		WritePaths:    []string{"./.goose/**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{},
	}

	engine := NewDecisionEngine(policy)

	// Act & Assert: Create operations should use write_paths
	tests := []struct {
		name      string
		path      string
		operation Operation
		expected  Decision
	}{
		{
			name:      "create in allowed write path",
			path:      "./.goose/newfile.txt",
			operation: OperationCreate,
			expected:  DecisionAllow,
		},
		{
			name:      "create outside write path",
			path:      "./other/newfile.txt",
			operation: OperationCreate,
			expected:  DecisionAsk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.CheckAccess(tt.path, tt.operation)
			assert.Equal(t, tt.expected, result.Decision)
		})
	}
}

// TestDecisionEngine_DeleteOperation tests file deletion operations
func TestDecisionEngine_DeleteOperation(t *testing.T) {
	// Arrange: Create policy
	policy := &SecurityPolicy{
		WritePaths:    []string{"./.goose/**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{},
	}

	engine := NewDecisionEngine(policy)

	// Act & Assert: Delete operations should use write_paths
	tests := []struct {
		name      string
		path      string
		operation Operation
		expected  Decision
	}{
		{
			name:      "delete in allowed write path",
			path:      "./.goose/file.txt",
			operation: OperationDelete,
			expected:  DecisionAllow,
		},
		{
			name:      "delete outside write path",
			path:      "./other/file.txt",
			operation: OperationDelete,
			expected:  DecisionAsk,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.CheckAccess(tt.path, tt.operation)
			assert.Equal(t, tt.expected, result.Decision)
		})
	}
}

// TestDecisionEngine_ThreeStageFlow tests the complete 3-stage decision flow
func TestDecisionEngine_ThreeStageFlow(t *testing.T) {
	// Arrange: Create policy with all three stages
	policy := &SecurityPolicy{
		WritePaths:    []string{"./.goose/**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{"/**/.env"},
	}

	engine := NewDecisionEngine(policy)

	// Test 1: Blocked takes precedence (even if in write_paths)
	t.Run("blocked takes precedence", func(t *testing.T) {
		result := engine.CheckAccess("/.goose/.env", OperationWrite)
		assert.Equal(t, DecisionDeny, result.Decision)
		assert.Contains(t, result.Reason, "blocked_always")
	})

	// Test 2: Write paths checked for write operations
	t.Run("write paths allow write", func(t *testing.T) {
		result := engine.CheckAccess("./.goose/config.yaml", OperationWrite)
		assert.Equal(t, DecisionAllow, result.Decision)
		assert.Contains(t, result.Reason, "write_paths")
	})

	// Test 3: Read paths checked for read operations
	t.Run("read paths allow read", func(t *testing.T) {
		result := engine.CheckAccess("./file.txt", OperationRead)
		assert.Equal(t, DecisionAllow, result.Decision)
		assert.Contains(t, result.Reason, "read_paths")
	})

	// Test 4: Ask when no match
	t.Run("ask when no match", func(t *testing.T) {
		result := engine.CheckAccess("/tmp/file.txt", OperationWrite)
		assert.Equal(t, DecisionAsk, result.Decision)
		assert.Contains(t, result.Reason, "user confirmation required")
	})
}

// TestDecisionEngine_AccessResultStructure tests the AccessResult structure
func TestDecisionEngine_AccessResultStructure(t *testing.T) {
	// Arrange
	policy := &SecurityPolicy{
		WritePaths:    []string{"./**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{},
	}
	engine := NewDecisionEngine(policy)

	// Act
	result := engine.CheckAccess("./file.txt", OperationRead)

	// Assert: Verify all fields are populated
	assert.NotEmpty(t, result.Reason, "Reason should not be empty")
	assert.NotEmpty(t, result.Policy, "Policy should indicate which rule matched")
	assert.Contains(t, result.Reason, "read_paths", "Reason should mention read_paths")
	assert.True(t, result.Decision == DecisionAllow || result.Decision == DecisionDeny || result.Decision == DecisionAsk,
		"Decision should be one of Allow/Deny/Ask")
}
