package fsaccess

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyReloader_ReloadNow(t *testing.T) {
	// Arrange: Create a temporary security.yaml
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.yaml")

	initialContent := []byte("write_paths:\n  - ./src/**\nread_paths:\n  - ./**\nblocked_always:\n  - ~/.ssh/**\n")
	err := os.WriteFile(configPath, initialContent, 0644)
	require.NoError(t, err)

	policy, err := LoadSecurityPolicy(configPath)
	require.NoError(t, err)

	engine := NewDecisionEngine(policy)
	reloader, err := NewPolicyReloader(configPath, engine, 1*time.Second, nil)
	require.NoError(t, err)

	// Act: Force reload
	err = reloader.ReloadNow()
	require.NoError(t, err)

	// Assert: Reload count incremented
	assert.Equal(t, int64(1), reloader.ReloadCount())
}

func TestPolicyReloader_DetectsFileChange(t *testing.T) {
	// Arrange: Create initial config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.yaml")

	initialContent := []byte("write_paths:\n  - ./src/**\nread_paths:\n  - ./**\nblocked_always: []\n")
	err := os.WriteFile(configPath, initialContent, 0644)
	require.NoError(t, err)

	policy, err := LoadSecurityPolicy(configPath)
	require.NoError(t, err)

	engine := NewDecisionEngine(policy)
	reloader, err := NewPolicyReloader(configPath, engine, 100*time.Millisecond, nil)
	require.NoError(t, err)

	// Start reloader
	reloader.Start()
	defer reloader.Stop()

	// Act: Modify file after a short delay
	time.Sleep(50 * time.Millisecond)
	updatedContent := []byte("write_paths:\n  - ./src/**\n  - ./docs/**\nread_paths:\n  - ./**\nblocked_always: []\n")
	err = os.WriteFile(configPath, updatedContent, 0644)
	require.NoError(t, err)

	// Wait for reload to detect change
	time.Sleep(300 * time.Millisecond)

	// Assert: At least one reload happened
	assert.GreaterOrEqual(t, reloader.ReloadCount(), int64(1))
}

func TestPolicyReloader_EmptyPath(t *testing.T) {
	policy := &SecurityPolicy{
		WritePaths:    []string{"./**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{},
	}
	engine := NewDecisionEngine(policy)

	_, err := NewPolicyReloader("", engine, 1*time.Second, nil)
	assert.Error(t, err)
}

func TestPolicyReloader_NilEngine(t *testing.T) {
	_, err := NewPolicyReloader("/some/path.yaml", nil, 1*time.Second, nil)
	assert.Error(t, err)
}

func TestPolicyReloader_NonexistentFile(t *testing.T) {
	_, err := NewPolicyReloader("/nonexistent/security.yaml", NewDecisionEngine(&SecurityPolicy{}), 1*time.Second, nil)
	assert.Error(t, err)
}

func TestPolicyReloader_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.yaml")

	content := []byte("write_paths: []\nread_paths: []\nblocked_always: []\n")
	err := os.WriteFile(configPath, content, 0644)
	require.NoError(t, err)

	policy, err := LoadSecurityPolicy(configPath)
	require.NoError(t, err)

	engine := NewDecisionEngine(policy)
	reloader, err := NewPolicyReloader(configPath, engine, 100*time.Millisecond, nil)
	require.NoError(t, err)

	// Start and stop should work cleanly
	reloader.Start()
	time.Sleep(50 * time.Millisecond)
	reloader.Stop()

	// Double stop should not panic
	reloader.Stop()
}
