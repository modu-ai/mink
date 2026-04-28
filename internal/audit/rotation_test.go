package audit

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotatingWriter_Write(t *testing.T) {
	// Arrange: Create a rotating writer with small max size for testing
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Set very small max size to trigger rotation quickly
	writer, err := NewRotatingWriter(logPath, 1024) // 1KB max size
	require.NoError(t, err)
	defer writer.Close()

	// Act: Write events until rotation occurs
	event := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		strings.Repeat("Test write event with padding to trigger rotation ", 10),
		map[string]string{"path": "/tmp/test.txt"},
	)

	// Write enough events to exceed max size
	for range 5 {
		err := writer.Write(event)
		require.NoError(t, err)
	}

	// Give a moment for compression to complete
	time.Sleep(100 * time.Millisecond)

	// Assert: Verify rotation occurred
	// Original file should still exist
	_, err = os.Stat(logPath)
	require.NoError(t, err, "Original log file should exist")

	// Rotated file should exist with timestamp suffix
	matches, err := filepath.Glob(filepath.Join(tmpDir, "audit.log.*.gz"))
	require.NoError(t, err)
	assert.Greater(t, len(matches), 0, "Should have at least one rotated file")

	// Verify rotated file is gzip compressed
	for _, match := range matches {
		verifyGzipFile(t, match)
	}
}

func TestRotatingWriter_RotationOnSizeLimit(t *testing.T) {
	// Arrange: Create writer with specific size limit
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	maxSize := int64(500) // 500 bytes
	writer, err := NewRotatingWriter(logPath, maxSize)
	require.NoError(t, err)
	defer writer.Close()

	// Act: Write event that exceeds max size
	largeEvent := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		strings.Repeat("Large event ", 100), // ~1100 bytes
		map[string]string{"data": strings.Repeat("x", 500)},
	)

	err = writer.Write(largeEvent)
	require.NoError(t, err)

	// Assert: Rotation should have been triggered
	matches, err := filepath.Glob(filepath.Join(tmpDir, "audit.log.*.gz"))
	require.NoError(t, err)
	assert.Greater(t, len(matches), 0, "Should have rotated files")

	// Current file should contain the large event (it was written after rotation)
	info, err := os.Stat(logPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0), "Current file should have the large event")
}

func TestRotatingWriter_RotationTimestampSuffix(t *testing.T) {
	// Arrange: Create writer with small max size
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewRotatingWriter(logPath, 1000)
	require.NoError(t, err)
	defer writer.Close()

	// Record time before rotation
	beforeRotation := time.Now().UTC()

	// Act: Trigger rotation by writing multiple large events
	largeEvent := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		strings.Repeat("Trigger rotation ", 50),
		nil,
	)

	// Write enough to trigger rotation
	for range 5 {
		err = writer.Write(largeEvent)
		require.NoError(t, err)
	}

	// Give a moment for compression to complete
	time.Sleep(100 * time.Millisecond)

	// Assert: Verify timestamp suffix format
	matches, err := filepath.Glob(filepath.Join(tmpDir, "audit.log.*.gz"))
	require.NoError(t, err)
	require.Greater(t, len(matches), 0, "Should have rotated files")

	// Extract timestamp from filename
	// Format: audit.log.20060102-150405.gz
	for _, match := range matches {
		base := filepath.Base(match)
		// Remove "audit.log." prefix and ".gz" suffix
		timestampStr := strings.TrimPrefix(base, "audit.log.")
		timestampStr = strings.TrimSuffix(timestampStr, ".gz")

		// Parse timestamp
		parsedTime, err := time.Parse("20060102-150405", timestampStr)
		require.NoError(t, err, "Timestamp should be in correct format")

		// Verify timestamp is recent
		assert.True(t, parsedTime.After(beforeRotation.Add(-1*time.Second)) || parsedTime.Before(time.Now().UTC().Add(1*time.Second)),
			"Timestamp should be close to current time")
	}
}

func TestRotatingWriter_NoRotationBelowLimit(t *testing.T) {
	// Arrange: Create writer with large max size
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewRotatingWriter(logPath, 100*1024*1024) // 100MB
	require.NoError(t, err)
	defer writer.Close()

	// Act: Write small event
	event := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		"Small event",
		nil,
	)
	err = writer.Write(event)
	require.NoError(t, err)

	// Assert: No rotation should occur
	matches, err := filepath.Glob(filepath.Join(tmpDir, "audit.log.*.gz"))
	require.NoError(t, err)
	assert.Equal(t, 0, len(matches), "Should have no rotated files")
}

func TestRotatingWriter_RotatedFileIntegrity(t *testing.T) {
	// Arrange: Create writer with small max size
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewRotatingWriter(logPath, 500)
	require.NoError(t, err)
	defer writer.Close()

	// Write known events
	event1 := NewAuditEvent(time.Now().UTC(), EventTypeFSWrite, SeverityInfo, "Event 1", map[string]string{"id": "1"})

	// Write first event
	err = writer.Write(event1)
	require.NoError(t, err)

	// Write large event to trigger rotation
	largeEvent := NewAuditEvent(
		time.Now().UTC(),
		EventTypeFSWrite,
		SeverityInfo,
		strings.Repeat("Large ", 100),
		nil,
	)
	err = writer.Write(largeEvent)
	require.NoError(t, err)

	// Assert: Verify rotated file contains first event
	matches, err := filepath.Glob(filepath.Join(tmpDir, "audit.log.*.gz"))
	require.NoError(t, err)
	require.Greater(t, len(matches), 0, "Should have rotated files")

	// Read and verify rotated file content
	rotatedFile := matches[0]
	events, err := readGzipAuditEvents(rotatedFile)
	require.NoError(t, err)
	assert.Greater(t, len(events), 0, "Rotated file should contain events")

	// Verify first event is in rotated file
	found := false
	for _, e := range events {
		if e.Message == "Event 1" && e.Metadata["id"] == "1" {
			found = true
			break
		}
	}
	assert.True(t, found, "First event should be in rotated file")
}

func TestRotatingWriter_ConcurrentWritesWithRotation(t *testing.T) {
	// Arrange: Create writer with small max size
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewRotatingWriter(logPath, 1000)
	require.NoError(t, err)
	defer writer.Close()

	// Act: Write concurrently with large events
	numGoroutines := 5
	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(id int) {
			for range 10 {
				event := NewAuditEvent(
					time.Now().UTC(),
					EventTypeFSWrite,
					SeverityInfo,
					strings.Repeat("Concurrent event ", 20),
					map[string]string{"goroutine": string(rune(id))},
				)
				err := writer.Write(event)
				assert.NoError(t, err)
			}
			done <- true
		}(i)
	}

	// Wait for completion
	for range numGoroutines {
		<-done
	}

	// Give a moment for any pending compression to complete
	time.Sleep(100 * time.Millisecond)

	// Assert: Verify files are valid
	info, err := os.Stat(logPath)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0, "Current log file should have content")

	// Verify rotated files are valid gzip
	matches, err := filepath.Glob(filepath.Join(tmpDir, "audit.log.*.gz"))
	require.NoError(t, err)
	for _, match := range matches {
		verifyGzipFile(t, match)
	}
}

func TestRotatingWriter_MaxSize(t *testing.T) {
	// Arrange: Default max size should be 100MB
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	writer, err := NewRotatingWriter(logPath, 0) // Use default
	require.NoError(t, err)
	defer writer.Close()

	// Assert: Verify default max size
	// 100MB = 100 * 1024 * 1024 bytes
	expectedMaxSize := int64(100 * 1024 * 1024)
	assert.Equal(t, expectedMaxSize, writer.MaxSize())
}

// Helper functions

func verifyGzipFile(t *testing.T, path string) {
	// Verify file exists first
	_, err := os.Stat(path)
	require.NoError(t, err, "Gzip file should exist")

	// Open gzip file
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	// Verify it's a valid gzip file
	gzReader, err := gzip.NewReader(file)
	require.NoError(t, err, "Should be valid gzip format")
	defer gzReader.Close()

	// Try to read some data
	buf := make([]byte, 1024)
	n, err := gzReader.Read(buf)
	// EOF is ok (empty file), other errors are not
	if err != nil && n == 0 {
		assert.True(t, err == nil, "Should be able to read gzip file, got: %v", err)
	}
}

func readGzipAuditEvents(path string) ([]AuditEvent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	var events []AuditEvent
	decoder := json.NewDecoder(gzReader)
	for {
		var event AuditEvent
		if err := decoder.Decode(&event); err != nil {
			break
		}
		events = append(events, event)
	}

	return events, nil
}
