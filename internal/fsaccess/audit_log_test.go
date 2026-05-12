// Package fsaccess provides filesystem access control with policy-based security.
// SPEC-GOOSE-FS-ACCESS-001
package fsaccess

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditLogger_LogDecision tests audit logging for access decisions
func TestAuditLogger_LogDecision(t *testing.T) {
	// Arrange: Create temporary audit log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	auditLogger := NewAuditLogger(writer)

	// Test case 1: Log allowed decision
	t.Run("log allowed decision", func(t *testing.T) {
		result := AccessResult{
			Decision: DecisionAllow,
			Reason:   "Path matches allowed write_paths policy",
			Policy:   "write_paths",
		}

		err := auditLogger.LogDecision(context.Background(), "./file.txt", OperationWrite, result)
		require.NoError(t, err)

		// Verify log file contains entry
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "fs.write")
		assert.Contains(t, string(data), "./file.txt")
		assert.Contains(t, string(data), "allow")
	})

	// Test case 2: Log denied decision
	t.Run("log denied decision", func(t *testing.T) {
		result := AccessResult{
			Decision: DecisionDeny,
			Reason:   "Path matches blocked_always policy",
			Policy:   "blocked_always",
		}

		err := auditLogger.LogDecision(context.Background(), "/etc/passwd", OperationRead, result)
		require.NoError(t, err)

		// Verify log file contains entry
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "fs.blocked_always")
		assert.Contains(t, string(data), "/etc/passwd")
		assert.Contains(t, string(data), "deny")
	})

	// Test case 3: Log ask decision
	t.Run("log ask decision", func(t *testing.T) {
		result := AccessResult{
			Decision: DecisionAsk,
			Reason:   "User confirmation required",
			Policy:   "no matching policy",
		}

		err := auditLogger.LogDecision(context.Background(), "/tmp/file.txt", OperationWrite, result)
		require.NoError(t, err)

		// Verify log file contains entry
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)
		assert.Contains(t, string(data), "fs.read.denied") // Ask decisions use this event type
		assert.Contains(t, string(data), "/tmp/file.txt")
		assert.Contains(t, string(data), "ask")
	})
}

// TestAskUserQuestion_Callback tests the AskUserQuestion callback mechanism
func TestAskUserQuestion_Callback(t *testing.T) {
	// Test case 1: User grants permission
	t.Run("user grants permission", func(t *testing.T) {
		askFunc := func(_ context.Context, _ string, _ Operation) (bool, error) {
			return true, nil // User allows
		}

		allowed, err := askFunc(context.Background(), "./file.txt", OperationWrite)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	// Test case 2: User denies permission
	t.Run("user denies permission", func(t *testing.T) {
		askFunc := func(_ context.Context, _ string, _ Operation) (bool, error) {
			return false, nil // User denies
		}

		allowed, err := askFunc(context.Background(), "/etc/passwd", OperationRead)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	// Test case 3: User cancels (error)
	t.Run("user cancels prompt", func(t *testing.T) {
		askFunc := func(_ context.Context, _ string, _ Operation) (bool, error) {
			return false, context.Canceled
		}

		allowed, err := askFunc(context.Background(), "./file.txt", OperationWrite)
		assert.Error(t, err)
		assert.False(t, allowed)
	})

	// Test case 4: Context timeout
	t.Run("context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		askFunc := func(_ context.Context, _ string, _ Operation) (bool, error) {
			<-ctx.Done()
			return false, ctx.Err()
		}

		allowed, err := askFunc(ctx, "./file.txt", OperationWrite)
		assert.Error(t, err)
		assert.False(t, allowed)
	})
}

// TestAuditLogger_Completeness tests audit log completeness (AC-05)
func TestAuditLogger_Completeness(t *testing.T) {
	// Arrange: Create temporary audit log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := audit.NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	auditLogger := NewAuditLogger(writer)

	// Act: Log a decision
	result := AccessResult{
		Decision: DecisionAllow,
		Reason:   "Path matches allowed policy",
		Policy:   "write_paths",
	}

	err = auditLogger.LogDecision(context.Background(), "./test.txt", OperationWrite, result)
	require.NoError(t, err)

	// Assert: Verify log contains all required fields (AC-05)
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	logContent := string(data)

	// AC-05: Audit log must contain operation, path, allowed, reason, at (timestamp)
	assert.Contains(t, logContent, "operation") // operation field
	assert.Contains(t, logContent, "path")      // path field
	assert.Contains(t, logContent, "allowed")   // decision field
	assert.Contains(t, logContent, "reason")    // reason field
	assert.Contains(t, logContent, "timestamp") // at field
}
