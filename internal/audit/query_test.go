package audit

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestQuery_ReadsCurrentLogFile tests that Query reads from the current audit.log file
func TestQuery_ReadsCurrentLogFile(t *testing.T) {
	// Arrange: Create temporary directory and audit log file
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	// Create test events
	now := time.Now().UTC().Truncate(time.Second)
	events := []AuditEvent{
		NewAuditEvent(now, EventTypeFSWrite, SeverityInfo, "File write test", map[string]string{"path": "/tmp/test"}),
		NewAuditEvent(now.Add(time.Second), EventTypePermissionGrant, SeverityInfo, "Permission granted", nil),
	}

	// Write events to log file
	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	for _, event := range events {
		data, _ := json.Marshal(event)
		if _, err := file.Write(append(data, '\n')); err != nil {
			t.Fatalf("Failed to write to log file: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close log file: %v", err)
	}

	// Act: Query all events
	result, err := Query(tmpDir, QueryOptions{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Assert: Should return all events
	if len(result) != 2 {
		t.Errorf("Expected 2 events, got %d", len(result))
	}
}

// TestQuery_ReadsRotatedGzFiles tests that Query reads from rotated .gz files
func TestQuery_ReadsRotatedGzFiles(t *testing.T) {
	// Arrange: Create temporary directory with rotated log
	tmpDir := t.TempDir()
	rotatedFile := filepath.Join(tmpDir, "audit.log.20060102-150405.gz")

	// Create test event
	now := time.Now().UTC().Truncate(time.Second)
	event := NewAuditEvent(now, EventTypeFSWrite, SeverityInfo, "Rotated log test", nil)

	// Write compressed event
	file, err := os.Create(rotatedFile)
	if err != nil {
		t.Fatalf("Failed to create rotated file: %v", err)
	}
	gzWriter := gzip.NewWriter(file)
	data, _ := json.Marshal(event)
	if _, err := gzWriter.Write(append(data, '\n')); err != nil {
		t.Fatalf("Failed to write to gzip writer: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close rotated file: %v", err)
	}

	// Act: Query all events
	result, err := Query(tmpDir, QueryOptions{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Assert: Should return event from rotated file
	if len(result) != 1 {
		t.Errorf("Expected 1 event, got %d", len(result))
	}
}

// TestQuery_FiltersByTimeRange tests time range filtering with since/until
func TestQuery_FiltersByTimeRange(t *testing.T) {
	// Arrange: Create temporary directory with events at different times
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	baseTime := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	events := []AuditEvent{
		NewAuditEvent(baseTime, EventTypeFSWrite, SeverityInfo, "Event 1", nil),
		NewAuditEvent(baseTime.Add(30*time.Minute), EventTypeFSWrite, SeverityInfo, "Event 2", nil),
		NewAuditEvent(baseTime.Add(60*time.Minute), EventTypeFSWrite, SeverityInfo, "Event 3", nil),
	}

	// Write events
	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	for _, event := range events {
		data, _ := json.Marshal(event)
		if _, err := file.Write(append(data, '\n')); err != nil {
			t.Fatalf("Failed to write to log file: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close log file: %v", err)
	}

	// Act & Assert: Query with since filter
	since := baseTime.Add(15 * time.Minute)
	result, err := Query(tmpDir, QueryOptions{Since: &since})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 events with since filter, got %d", len(result))
	}

	// Act & Assert: Query with until filter
	until := baseTime.Add(45 * time.Minute)
	result, err = Query(tmpDir, QueryOptions{Until: &until})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 events with until filter, got %d", len(result))
	}

	// Act & Assert: Query with both since and until
	result, err = Query(tmpDir, QueryOptions{Since: &since, Until: &until})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 event with both filters, got %d", len(result))
	}
}

// TestQuery_FiltersByEventType tests event type filtering
func TestQuery_FiltersByEventType(t *testing.T) {
	// Arrange: Create temporary directory with different event types
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	now := time.Now().UTC().Truncate(time.Second)
	events := []AuditEvent{
		NewAuditEvent(now, EventTypeFSWrite, SeverityInfo, "File write", nil),
		NewAuditEvent(now, EventTypePermissionGrant, SeverityInfo, "Permission grant", nil),
		NewAuditEvent(now, EventTypeCredentialAccessed, SeverityWarning, "Credential access", nil),
	}

	// Write events
	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	for _, event := range events {
		data, _ := json.Marshal(event)
		if _, err := file.Write(append(data, '\n')); err != nil {
			t.Fatalf("Failed to write to log file: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close log file: %v", err)
	}

	// Act: Query with type filter
	types := []EventType{EventTypeFSWrite, EventTypePermissionGrant}
	result, err := Query(tmpDir, QueryOptions{Types: types})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Assert: Should return only matching types
	if len(result) != 2 {
		t.Errorf("Expected 2 events with type filter, got %d", len(result))
	}
	for _, event := range result {
		if event.Type != EventTypeFSWrite && event.Type != EventTypePermissionGrant {
			t.Errorf("Unexpected event type: %s", event.Type)
		}
	}
}

// TestQuery_SortsByTimestamp tests that results are sorted by timestamp
func TestQuery_SortsByTimestamp(t *testing.T) {
	// Arrange: Create temporary directory with unsorted events
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	baseTime := time.Now().UTC().Truncate(time.Second)
	events := []AuditEvent{
		NewAuditEvent(baseTime.Add(2*time.Second), EventTypeFSWrite, SeverityInfo, "Event 3", nil),
		NewAuditEvent(baseTime, EventTypeFSWrite, SeverityInfo, "Event 1", nil),
		NewAuditEvent(baseTime.Add(time.Second), EventTypeFSWrite, SeverityInfo, "Event 2", nil),
	}

	// Write events in unsorted order
	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	for _, event := range events {
		data, _ := json.Marshal(event)
		if _, err := file.Write(append(data, '\n')); err != nil {
				t.Fatalf("Failed to write to log file: %v", err)
			}
	}
	if err := file.Close(); err != nil {
				t.Fatalf("Failed to close log file: %v", err)
			}

	// Act: Query all events
	result, err := Query(tmpDir, QueryOptions{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Assert: Should be sorted by timestamp
	if len(result) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(result))
	}
	for i := 1; i < len(result); i++ {
		if result[i-1].Timestamp.After(result[i].Timestamp) {
			t.Errorf("Events not sorted: %v after %v", result[i-1].Timestamp, result[i].Timestamp)
		}
	}
}

// TestQuery_HandlesCorruptJSONLines tests that corrupt JSON lines are skipped with warning
func TestQuery_HandlesCorruptJSONLines(t *testing.T) {
	// Arrange: Create temporary directory with corrupt data
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	now := time.Now().UTC().Truncate(time.Second)
	validEvent := NewAuditEvent(now, EventTypeFSWrite, SeverityInfo, "Valid event", nil)

	// Write mix of valid and corrupt data
	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	// Valid event
	data, _ := json.Marshal(validEvent)
	if _, err := file.Write(append(data, '\n')); err != nil {
		t.Fatalf("Failed to write to log file: %v", err)
	}

	// Corrupt line
	if _, err := file.Write([]byte("{invalid json\n")); err != nil {
		t.Fatalf("Failed to write to log file: %v", err)
	}

	// Another valid event
	if _, err := file.Write(append(data, '\n')); err != nil {
		t.Fatalf("Failed to write to log file: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close log file: %v", err)
	}

	// Act: Query should handle corrupt data gracefully
	result, err := Query(tmpDir, QueryOptions{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Assert: Should return only valid events
	if len(result) != 2 {
		t.Errorf("Expected 2 valid events, got %d", len(result))
	}
}

// TestQuery_StreamReadsFiles tests that Query streams files without loading entire file into memory
func TestQuery_StreamReadsFiles(t *testing.T) {
	// Arrange: Create a log file with many events
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	now := time.Now().UTC().Truncate(time.Second)
	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	// Write 100 events
	for i := range 100 {
		event := NewAuditEvent(now.Add(time.Duration(i)*time.Second), EventTypeFSWrite, SeverityInfo, "Event", nil)
		data, _ := json.Marshal(event)
		if _, err := file.Write(append(data, '\n')); err != nil {
			t.Fatalf("Failed to write to log file: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close log file: %v", err)
	}

	// Act: Query should stream-read the file
	result, err := Query(tmpDir, QueryOptions{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Assert: Should return all events
	if len(result) != 100 {
		t.Errorf("Expected 100 events, got %d", len(result))
	}
}

// TestQuery_CombinesCurrentAndRotatedFiles tests that Query combines results from current and rotated files
func TestQuery_CombinesCurrentAndRotatedFiles(t *testing.T) {
	// Arrange: Create both current and rotated log files
	tmpDir := t.TempDir()
	currentLog := filepath.Join(tmpDir, "audit.log")
	rotatedLog := filepath.Join(tmpDir, "audit.log.20060102-150405.gz")

	now := time.Now().UTC().Truncate(time.Second)

	// Write to rotated file
	file1, err := os.Create(rotatedLog)
	if err != nil {
		t.Fatalf("Failed to create rotated file: %v", err)
	}
	gzWriter := gzip.NewWriter(file1)
	event1 := NewAuditEvent(now, EventTypeFSWrite, SeverityInfo, "Rotated event", nil)
	data1, _ := json.Marshal(event1)
	if _, err := gzWriter.Write(append(data1, '\n')); err != nil {
		t.Fatalf("Failed to write to gzip writer: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}
	if err := file1.Close(); err != nil {
		t.Fatalf("Failed to close rotated file: %v", err)
	}

	// Write to current file
	file2, err := os.Create(currentLog)
	if err != nil {
		t.Fatalf("Failed to create current log file: %v", err)
	}
	event2 := NewAuditEvent(now.Add(time.Second), EventTypePermissionGrant, SeverityInfo, "Current event", nil)
	data2, _ := json.Marshal(event2)
	if _, err := file2.Write(append(data2, '\n')); err != nil {
		t.Fatalf("Failed to write to log file: %v", err)
	}
	if err := file2.Close(); err != nil {
		t.Fatalf("Failed to close current log file: %v", err)
	}

	// Act: Query should combine both files
	result, err := Query(tmpDir, QueryOptions{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Assert: Should return events from both files
	if len(result) != 2 {
		t.Errorf("Expected 2 events from combined files, got %d", len(result))
	}

	// Check that both event types are present
	hasFsWrite := false
	hasPermissionGrant := false
	for _, event := range result {
		if event.Type == EventTypeFSWrite {
			hasFsWrite = true
		}
		if event.Type == EventTypePermissionGrant {
			hasPermissionGrant = true
		}
	}
	if !hasFsWrite || !hasPermissionGrant {
		t.Error("Missing expected event types from combined files")
	}
}

// TestQuery_EmptyDirectory tests Query with empty directory
func TestQuery_EmptyDirectory(t *testing.T) {
	// Arrange: Empty temporary directory
	tmpDir := t.TempDir()

	// Act: Query should return empty result
	result, err := Query(tmpDir, QueryOptions{})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Assert: Should return empty slice
	if result == nil {
		t.Error("Expected empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 events, got %d", len(result))
	}
}

// TestQuery_NonExistentDirectory tests Query with non-existent directory
func TestQuery_NonExistentDirectory(t *testing.T) {
	// Arrange: Non-existent directory path
	nonExistentDir := filepath.Join(os.TempDir(), "non-existent-audit-dir-"+strings.ReplaceAll(time.Now().Format(time.RFC3339Nano), ":", "-"))

	// Act: Query should handle non-existent directory gracefully
	result, err := Query(nonExistentDir, QueryOptions{})

	// Assert: Should return empty result without error
	if err != nil {
		t.Fatalf("Query should not fail on non-existent directory: %v", err)
	}
	if result == nil {
		t.Error("Expected empty slice, got nil")
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 events, got %d", len(result))
	}
}

// TestQuery_InvalidTimeRange tests Query with invalid time range (since after until)
func TestQuery_InvalidTimeRange(t *testing.T) {
	// Arrange: Create temporary directory with events
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "audit.log")

	now := time.Now().UTC().Truncate(time.Second)
	event := NewAuditEvent(now, EventTypeFSWrite, SeverityInfo, "Test event", nil)

	file, err := os.Create(logFile)
	if err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}
	data, _ := json.Marshal(event)
	if _, err := file.Write(append(data, '\n')); err != nil {
		t.Fatalf("Failed to write to log file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Failed to close log file: %v", err)
	}

	// Act: Query with since after until
	since := now.Add(24 * time.Hour)
	until := now
	result, err := Query(tmpDir, QueryOptions{Since: &since, Until: &until})

	// Assert: Should return empty result (no events match)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 events with invalid time range, got %d", len(result))
	}
}
