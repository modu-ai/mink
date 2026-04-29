package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDualWriter_Write(t *testing.T) {
	// Arrange: Create dual writer with local enabled
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: true,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	event := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		"Test event",
		nil,
	)

	// Act: Write event
	err = writer.Write(event)

	// Assert: Both files should exist
	require.NoError(t, err)

	_, err = os.Stat(globalPath)
	require.NoError(t, err, "Global log should exist")

	_, err = os.Stat(localPath)
	require.NoError(t, err, "Local log should exist")
}

func TestDualWriter_Write_LocalDisabled(t *testing.T) {
	// Arrange: Create dual writer with local disabled
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: false,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	event := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		"Test event",
		nil,
	)

	// Act: Write event
	err = writer.Write(event)

	// Assert: Only global file should exist
	require.NoError(t, err)

	_, err = os.Stat(globalPath)
	require.NoError(t, err, "Global log should exist")

	_, err = os.Stat(localPath)
	assert.True(t, os.IsNotExist(err), "Local log should not exist when disabled")
}

func TestDualWriter_IsLocalEnabled(t *testing.T) {
	// Arrange: Create dual writer with local enabled
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: true,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	// Assert: Local should be enabled
	assert.True(t, writer.IsLocalEnabled(), "Local logging should be enabled")
}

func TestDualWriter_Close(t *testing.T) {
	// Arrange: Create dual writer
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: true,
	})
	require.NoError(t, err)

	// Act: Close writer
	err = writer.Close()

	// Assert: Close should succeed
	require.NoError(t, err)
}

func TestDefaultGlobalAuditPath(t *testing.T) {
	// Set GOOSE_HOME for test
	oldHome := os.Getenv("GOOSE_HOME")
	defer func() { _ = os.Setenv("GOOSE_HOME", oldHome) }()

	_ = os.Setenv("GOOSE_HOME", "/test/goose")

	path, err := DefaultGlobalAuditPath()

	require.NoError(t, err)
	assert.Equal(t, "/test/goose/logs/audit.log", path)
}

func TestDefaultLocalAuditPath(t *testing.T) {
	path := DefaultLocalAuditPath()
	assert.Equal(t, ".goose/logs/audit.local.log", path)
}

func TestDualWriter_GlobalPath(t *testing.T) {
	// Arrange: Create dual writer with local enabled
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: true,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	// Act: Get global path
	result := writer.GlobalPath()

	// Assert: Should match configured path
	assert.Equal(t, globalPath, result)
}

func TestDualWriter_LocalPath(t *testing.T) {
	// Arrange: Create dual writer with local enabled
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: true,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	// Act: Get local path
	result := writer.LocalPath()

	// Assert: Should match configured path
	assert.Equal(t, localPath, result)
}

func TestDualWriter_LocalPath_Disabled(t *testing.T) {
	// Arrange: Create dual writer with local disabled
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: false,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	// Act: Get local path when disabled
	result := writer.LocalPath()

	// Assert: Should return empty string
	assert.Empty(t, result, "Local path should be empty when disabled")
}

func TestDualWriter_GlobalPath_NilWriter(t *testing.T) {
	// Arrange: Create nil dual writer
	var writer *DualWriter

	// Act: Get global path from nil writer
	result := writer.GlobalPath()

	// Assert: Should return empty string
	assert.Empty(t, result, "Global path should be empty for nil writer")
}

func TestDualWriter_LocalPath_NilWriter(t *testing.T) {
	// Arrange: Create nil dual writer
	var writer *DualWriter

	// Act: Get local path from nil writer
	result := writer.LocalPath()

	// Assert: Should return empty string
	assert.Empty(t, result, "Local path should be empty for nil writer")
}

func TestDualWriter_Write_ErrorPaths(t *testing.T) {
	// Test write error when global writer fails
	// Arrange: Create dual writer
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: true,
	})
	require.NoError(t, err)
	defer func() { _ = writer.Close() }()

	// Close global writer to simulate write failure
	require.NoError(t, writer.globalWriter.Close())

	// Act: Try to write after global writer is closed
	event := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		"Test event",
		nil,
	)
	err = writer.Write(event)

	// Assert: Should fail
	assert.Error(t, err, "Write should fail when global writer is closed")
}

func TestDualWriter_Close_ErrorPaths(t *testing.T) {
	// Arrange: Create dual writer
	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "global", "audit.log")
	localPath := filepath.Join(tmpDir, "local", "audit.local.log")

	writer, err := NewDualWriter(DualWriterConfig{
		GlobalPath:  globalPath,
		LocalPath:   localPath,
		MaxSize:     1000,
		EnableLocal: true,
	})
	require.NoError(t, err)

	// Manually close global writer to simulate error
	// Close it twice - first should succeed, second will return error
	require.NoError(t, writer.globalWriter.Close())

	// Act: Close dual writer (global writer already closed)
	err = writer.Close()

	// Assert: Should still succeed or return error (depends on implementation)
	// The implementation collects errors but doesn't fail if one component fails
	assert.NoError(t, err, "Close should handle already-closed writers gracefully")
}

func TestDefaultGlobalAuditPath_NoGOOSE_HOME(t *testing.T) {
	// Arrange: Unset GOOSE_HOME
	oldHome := os.Getenv("GOOSE_HOME")
	defer func() { _ = os.Setenv("GOOSE_HOME", oldHome) }()

	_ = os.Unsetenv("GOOSE_HOME")

	// Act: Get default path
	path, err := DefaultGlobalAuditPath()

	// Assert: Should use default ~/.goose/logs/audit.log
	require.NoError(t, err)
	homeDir, _ := os.UserHomeDir()
	expectedPath := filepath.Join(homeDir, ".goose", "logs", "audit.log")
	assert.Equal(t, expectedPath, path)
}
