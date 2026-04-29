package fsaccess

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAC01_ThreeStageDecisionFlow validates the 3-stage decision flow:
// blocked_always > write_paths/read_paths > AskUserQuestion
func TestAC01_ThreeStageDecisionFlow(t *testing.T) {
	policy := &SecurityPolicy{
		WritePaths:    []string{"./src/**", "./docs/**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{"~/.ssh/**", "/etc/**"},
	}
	engine := NewDecisionEngine(policy)

	// Stage 1: blocked_always takes highest priority
	result := engine.CheckAccess("/etc/passwd", OperationRead)
	assert.Equal(t, DecisionDeny, result.Decision, "blocked_always should deny unconditionally")
	assert.Equal(t, "blocked_always", result.Policy)

	// Stage 1 applies even if path would match allowed paths
	result = engine.CheckAccess("/etc/config", OperationRead)
	assert.Equal(t, DecisionDeny, result.Decision, "blocked_always overrides read_paths")

	// Stage 2: write_paths match
	result = engine.CheckAccess("./src/main.go", OperationWrite)
	assert.Equal(t, DecisionAllow, result.Decision, "write_paths should allow")
	assert.Equal(t, "write_paths", result.Policy)

	// Stage 2: read_paths match
	result = engine.CheckAccess("./README.md", OperationRead)
	assert.Equal(t, DecisionAllow, result.Decision, "read_paths should allow")
	assert.Equal(t, "read_paths", result.Policy)

	// Stage 3: no match → Ask
	result = engine.CheckAccess("/tmp/scratch.txt", OperationWrite)
	assert.Equal(t, DecisionAsk, result.Decision, "unmatched path should require user confirmation")
	assert.Equal(t, "no matching policy", result.Policy)
}

// TestAC02_GlobPatternMatching validates glob pattern support (recursive **, single *, ?)
func TestAC02_GlobPatternMatching(t *testing.T) {
	policy := &SecurityPolicy{
		WritePaths:    []string{"./src/**/*.go"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{},
	}
	engine := NewDecisionEngine(policy)

	// ** matches nested directories
	assert.Equal(t, DecisionAllow, engine.CheckAccess("./src/pkg/handler.go", OperationWrite).Decision)
	assert.Equal(t, DecisionAllow, engine.CheckAccess("./src/cmd/app/main.go", OperationWrite).Decision)

	// ** does not match when prefix differs
	assert.Equal(t, DecisionAsk, engine.CheckAccess("./test/handler.go", OperationWrite).Decision)

	// * matches single path segment
	policy2 := &SecurityPolicy{
		WritePaths:    []string{"./src/*"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{},
	}
	engine2 := NewDecisionEngine(policy2)
	assert.Equal(t, DecisionAllow, engine2.CheckAccess("./src/main.go", OperationWrite).Decision)
}

// TestAC03_BlockedAlwaysOverride validates that blocked_always cannot be overridden
func TestAC03_BlockedAlwaysOverride(t *testing.T) {
	policy := &SecurityPolicy{
		WritePaths:    []string{"./**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{"./secrets/**"},
	}
	engine := NewDecisionEngine(policy)

	// Even though ./** matches everything, blocked_always wins
	result := engine.CheckAccess("./secrets/key.pem", OperationRead)
	assert.Equal(t, DecisionDeny, result.Decision, "blocked_always must override read_paths")
	assert.Contains(t, result.Reason, "cannot be overridden")

	result = engine.CheckAccess("./secrets/key.pem", OperationWrite)
	assert.Equal(t, DecisionDeny, result.Decision, "blocked_always must override write_paths")
}

// TestAC04_HotReload validates hot-reload of security.yaml
func TestAC04_HotReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "security.yaml")

	initial := []byte("write_paths:\n  - ./src/**\nread_paths:\n  - ./**\nblocked_always: []\n")
	err := os.WriteFile(configPath, initial, 0644)
	require.NoError(t, err)

	policy, err := LoadSecurityPolicy(configPath)
	require.NoError(t, err)

	engine := NewDecisionEngine(policy)

	// Before reload: ./src/** is allowed for write
	assert.Equal(t, DecisionAllow, engine.CheckAccess("./src/main.go", OperationWrite).Decision)
	// ./docs/** is not allowed for write
	assert.Equal(t, DecisionAsk, engine.CheckAccess("./docs/guide.md", OperationWrite).Decision)

	// Setup reloader
	reloader, err := NewPolicyReloader(configPath, engine, 100*time.Millisecond, nil)
	require.NoError(t, err)

	// Force reload with updated policy
	updated := []byte("write_paths:\n  - ./src/**\n  - ./docs/**\nread_paths:\n  - ./**\nblocked_always: []\n")
	err = os.WriteFile(configPath, updated, 0644)
	require.NoError(t, err)

	err = reloader.ReloadNow()
	require.NoError(t, err)

	// After reload: ./docs/** is now allowed
	assert.Equal(t, DecisionAllow, engine.CheckAccess("./docs/guide.md", OperationWrite).Decision,
		"policy should reflect updated config after reload")
	assert.Equal(t, int64(1), reloader.ReloadCount())
}

// TestAC05_AuditLogCompleteness validates audit log contains all required fields
func TestAC05_AuditLogCompleteness(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	auditLogger := NewAuditLogger(writer)

	policy := &SecurityPolicy{
		WritePaths:    []string{"./**"},
		ReadPaths:     []string{"./**"},
		BlockedAlways: []string{"/etc/**"},
	}
	engine := NewDecisionEngine(policy)

	// Log allowed decision
	allowResult := engine.CheckAccess("./file.txt", OperationWrite)
	err = auditLogger.LogDecision(context.Background(), "./file.txt", OperationWrite, allowResult)
	require.NoError(t, err)

	// Log denied decision
	denyResult := engine.CheckAccess("/etc/shadow", OperationRead)
	err = auditLogger.LogDecision(context.Background(), "/etc/shadow", OperationRead, denyResult)
	require.NoError(t, err)

	// Log ask decision
	askResult := engine.CheckAccess("/tmp/file.txt", OperationWrite)
	err = auditLogger.LogDecision(context.Background(), "/tmp/file.txt", OperationWrite, askResult)
	require.NoError(t, err)

	// Verify all required fields are present
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logContent := string(data)

	// AC-05: operation, path, allowed, reason, at (timestamp)
	assert.Contains(t, logContent, "operation")
	assert.Contains(t, logContent, "path")
	assert.Contains(t, logContent, "allowed")
	assert.Contains(t, logContent, "reason")
	assert.Contains(t, logContent, "timestamp")
}
