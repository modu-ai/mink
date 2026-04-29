package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileWriter_Write(t *testing.T) {
	// Arrange: Create a temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewFileWriter(logPath)
	require.NoError(t, err)
	require.NotNil(t, writer)
	defer func() { require.NoError(t, writer.Close()) }()

	event := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		"Test write event",
		map[string]string{"path": "/tmp/test.txt"},
	)

	// Act: Write event
	err = writer.Write(event)

	// Assert: Verify write succeeded
	require.NoError(t, err)

	// Verify file exists and contains JSON line
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Verify JSON line format (should be valid JSON ending with newline)
	lines := splitLines(data)
	assert.Len(t, lines, 1, "Should have exactly one line")

	// Verify the line can be unmarshaled back to AuditEvent
	var unmarshaled AuditEvent
	err = unmarshalJSONLine(lines[0], &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, event.Type, unmarshaled.Type)
	assert.Equal(t, event.Severity, unmarshaled.Severity)
	assert.Equal(t, event.Message, unmarshaled.Message)
}

func TestFileWriter_WriteMultiple(t *testing.T) {
	// Arrange: Create a temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	// Act: Write multiple events
	events := []AuditEvent{
		NewAuditEvent(time.Now().UTC(), EventTypeFSWrite, SeverityInfo, "Event 1", nil),
		NewAuditEvent(time.Now().UTC(), EventTypePermissionGrant, SeverityWarning, "Event 2", nil),
		NewAuditEvent(time.Now().UTC(), EventTypeGoosedStart, SeverityInfo, "Event 3", nil),
	}

	for _, event := range events {
		err := writer.Write(event)
		require.NoError(t, err)
	}

	// Assert: Verify all events were written
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := splitLines(data)
	assert.Len(t, lines, 3, "Should have exactly three lines")

	// Verify each line is valid JSON
	for i, line := range lines {
		var event AuditEvent
		err := unmarshalJSONLine(line, &event)
		require.NoError(t, err, "Line %d should be valid JSON", i)
		assert.Equal(t, events[i].Type, event.Type)
	}
}

func TestFileWriter_AppendOnly(t *testing.T) {
	// Arrange: Create a log file with existing content
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Write initial content
	writer1, err := NewFileWriter(logPath)
	require.NoError(t, err)
	event1 := NewAuditEvent(time.Now().UTC(), EventTypeFSWrite, SeverityInfo, "First event", nil)
	err = writer1.Write(event1)
	require.NoError(t, err)
	require.NoError(t, writer1.Close())

	// Act: Create a new writer and append to the same file
	writer2, err := NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, writer2.Close()) }()

	event2 := NewAuditEvent(time.Now().UTC(), EventTypePermissionGrant, SeverityWarning, "Second event", nil)
	err = writer2.Write(event2)
	require.NoError(t, err)

	// Assert: Verify both events exist in file
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := splitLines(data)
	assert.Len(t, lines, 2, "Should have exactly two lines")
}

func TestFileWriter_ConcurrentWrites(t *testing.T) {
	// Arrange: Create a temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	// Act: Write concurrently from multiple goroutines
	numGoroutines := 10
	eventsPerGoroutine := 10

	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			for range eventsPerGoroutine {
				event := NewAuditEvent(
					time.Now().UTC(),
					EventTypeFSWrite,
					SeverityInfo,
					"Concurrent event",
					map[string]string{"goroutine": string(rune(id))},
				)
				err := writer.Write(event)
				assert.NoError(t, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}

	// Assert: Verify all events were written
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := splitLines(data)
	assert.Len(t, lines, numGoroutines*eventsPerGoroutine, "Should have all events")
}

func TestFileWriter_Close(t *testing.T) {
	// Arrange: Create a writer
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewFileWriter(logPath)
	require.NoError(t, err)

	// Act: Close the writer
	err = writer.Close()

	// Assert: Verify close succeeds
	require.NoError(t, err)

	// Verify we can still read the file
	_, err = os.Stat(logPath)
	require.NoError(t, err, "File should still exist after close")
}

func TestFileWriter_WriteAfterClose(t *testing.T) {
	// Arrange: Create and close a writer
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewFileWriter(logPath)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	// Act: Try to write after close
	event := NewAuditEvent(time.Now().UTC(), EventTypeFSWrite, SeverityInfo, "Test", nil)
	err = writer.Write(event)

	// Assert: Write should fail or be a no-op
	// Current implementation: file is closed, write will fail
	assert.Error(t, err, "Write after close should fail")
}

func TestFileWriter_CreateDirectory(t *testing.T) {
	// Arrange: Log file in a non-existent directory
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs", "audit.log")

	// Act: Create writer (should create parent directory)
	writer, err := NewFileWriter(logPath)

	// Assert: Should succeed
	require.NoError(t, err)
	require.NotNil(t, writer)
	defer func() { require.NoError(t, writer.Close()) }()

	// Verify directory was created
	dir := filepath.Dir(logPath)
	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "Parent directory should exist")
}

func TestFileWriter_Path(t *testing.T) {
	// Arrange: Create a temporary log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewFileWriter(logPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, writer.Close()) }()

	// Act: Get path
	result := writer.Path()

	// Assert: Should return configured path
	assert.Equal(t, logPath, result)
}

func TestFileWriter_Path_NilWriter(t *testing.T) {
	// Arrange: Create nil file writer
	var writer *FileWriter

	// Act: Get path from nil writer
	result := writer.Path()

	// Assert: Should return empty string
	assert.Empty(t, result, "Path should be empty for nil writer")
}

// Helper functions

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

func unmarshalJSONLine(line []byte, event *AuditEvent) error {
	return event.UnmarshalJSON(line)
}
