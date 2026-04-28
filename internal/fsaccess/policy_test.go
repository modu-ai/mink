// Package fsaccess provides filesystem access control with policy-based security.
// SPEC-GOOSE-FS-ACCESS-001
package fsaccess

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSecurityPolicy_LoadValidYAML tests loading a valid security policy from YAML file
func TestSecurityPolicy_LoadValidYAML(t *testing.T) {
	// Arrange: Create a temporary directory with a valid security.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.yaml")
	yamlContent := `
write_paths:
  - ./.goose/**
  - ./drafts/**/*.md
read_paths:
  - ./**
  - ~/.config/goose/**
blocked_always:
  - ~/.ssh/**
  - /etc/**
  - /var/**
  - ~/.gnupg/**
  - /**/.env
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Act: Load the security policy
	policy, err := LoadSecurityPolicy(configPath)

	// Assert: Verify policy is loaded correctly
	require.NoError(t, err)
	require.NotNil(t, policy)

	// Verify write_paths
	assert.Len(t, policy.WritePaths, 2)
	assert.Contains(t, policy.WritePaths, "./.goose/**")
	assert.Contains(t, policy.WritePaths, "./drafts/**/*.md")

	// Verify read_paths
	assert.Len(t, policy.ReadPaths, 2)
	assert.Contains(t, policy.ReadPaths, "./**")
	assert.Contains(t, policy.ReadPaths, "~/.config/goose/**")

	// Verify blocked_always
	assert.Len(t, policy.BlockedAlways, 5)
	assert.Contains(t, policy.BlockedAlways, "~/.ssh/**")
	assert.Contains(t, policy.BlockedAlways, "/etc/**")
	assert.Contains(t, policy.BlockedAlways, "/var/**")
	assert.Contains(t, policy.BlockedAlways, "~/.gnupg/**")
	assert.Contains(t, policy.BlockedAlways, "/**/.env")
}

// TestSecurityPolicy_LoadNonExistentFile tests error handling when config file doesn't exist
func TestSecurityPolicy_LoadNonExistentFile(t *testing.T) {
	// Arrange: Non-existent file path
	configPath := "/tmp/this-does-not-exist-security.yaml"

	// Act: Attempt to load the policy
	policy, err := LoadSecurityPolicy(configPath)

	// Assert: Should return error
	assert.Error(t, err)
	assert.Nil(t, policy)
}

// TestSecurityPolicy_LoadInvalidYAML tests error handling for malformed YAML
func TestSecurityPolicy_LoadInvalidYAML(t *testing.T) {
	// Arrange: Create file with invalid YAML
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.yaml")
	invalidYAML := `
write_paths:
  - ./.goose/**
  - ./drafts/**/*.md
read_paths: [
invalid yaml syntax here
`
	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	// Act: Attempt to load the policy
	policy, err := LoadSecurityPolicy(configPath)

	// Assert: Should return error
	assert.Error(t, err)
	assert.Nil(t, policy)
}

// TestSecurityPolicy_EmptyLists tests loading policy with empty path lists
func TestSecurityPolicy_EmptyLists(t *testing.T) {
	// Arrange: Create policy with empty lists
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.yaml")
	yamlContent := `
write_paths: []
read_paths: []
blocked_always: []
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Act: Load the policy
	policy, err := LoadSecurityPolicy(configPath)

	// Assert: Should load successfully with empty lists
	require.NoError(t, err)
	require.NotNil(t, policy)
	assert.Empty(t, policy.WritePaths)
	assert.Empty(t, policy.ReadPaths)
	assert.Empty(t, policy.BlockedAlways)
}

// TestSecurityPolicy_WithTildeExpansion tests that tilde paths are preserved as-is
func TestSecurityPolicy_WithTildeExpansion(t *testing.T) {
	// Arrange: Create policy with tilde paths
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.yaml")
	yamlContent := `
write_paths:
  - ~/Documents/**
read_paths:
  - ~/.config/goose/**
blocked_always:
  - ~/.ssh/**
`
	err := os.WriteFile(configPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Act: Load the policy
	policy, err := LoadSecurityPolicy(configPath)

	// Assert: Tilde should be preserved (expansion happens at match time)
	require.NoError(t, err)
	assert.Contains(t, policy.WritePaths, "~/Documents/**")
	assert.Contains(t, policy.ReadPaths, "~/.config/goose/**")
	assert.Contains(t, policy.BlockedAlways, "~/.ssh/**")
}
