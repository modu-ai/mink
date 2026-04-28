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
	defer writer.Close()

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
	defer writer.Close()

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
	defer writer.Close()

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
	defer os.Setenv("GOOSE_HOME", oldHome)

	os.Setenv("GOOSE_HOME", "/test/goose")

	path, err := DefaultGlobalAuditPath()

	require.NoError(t, err)
	assert.Equal(t, "/test/goose/logs/audit.log", path)
}

func TestDefaultLocalAuditPath(t *testing.T) {
	path := DefaultLocalAuditPath()
	assert.Equal(t, ".goose/logs/audit.local.log", path)
}
