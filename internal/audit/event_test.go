package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditEvent_MarshalJSON(t *testing.T) {
	// Arrange: Create a test event with known values
	timestamp := time.Date(2026, 4, 29, 12, 34, 56, 0, time.UTC)
	event := AuditEvent{
		Timestamp: timestamp,
		Type:      EventTypeFSWrite,
		Severity:  SeverityInfo,
		Message:   "Test write event",
		Metadata: map[string]string{
			"path":    "/tmp/test.txt",
			"size":    "1024",
			"subject": "skill:test-skill",
		},
	}

	// Act: Marshal to JSON
	data, err := json.Marshal(event)

	// Assert: Verify JSON structure
	require.NoError(t, err)
	require.NotNil(t, data)

	// Unmarshal to verify structure
	var unmarshaled map[string]any
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify required fields exist
	assert.Equal(t, "2026-04-29T12:34:56Z", unmarshaled["timestamp"])
	assert.Equal(t, "fs.write", unmarshaled["type"])
	assert.Equal(t, "info", unmarshaled["severity"])
	assert.Equal(t, "Test write event", unmarshaled["message"])

	// Verify metadata
	metadata, ok := unmarshaled["metadata"].(map[string]any)
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, "/tmp/test.txt", metadata["path"])
	assert.Equal(t, "1024", metadata["size"])
	assert.Equal(t, "skill:test-skill", metadata["subject"])
}

func TestAuditEvent_UnmarshalJSON(t *testing.T) {
	// Arrange: JSON string representing an audit event
	jsonStr := `{
		"timestamp": "2026-04-29T12:34:56Z",
		"type": "permission.grant",
		"severity": "warning",
		"message": "Permission granted",
		"metadata": {
			"subject_id": "skill:test",
			"capability": "fs.write",
			"scope": "/tmp"
		}
	}`

	// Act: Unmarshal to AuditEvent
	var event AuditEvent
	err := json.Unmarshal([]byte(jsonStr), &event)

	// Assert: Verify all fields are correctly unmarshaled
	require.NoError(t, err)
	assert.Equal(t, EventTypePermissionGrant, event.Type)
	assert.Equal(t, SeverityWarning, event.Severity)
	assert.Equal(t, "Permission granted", event.Message)

	// Verify metadata
	assert.Equal(t, "skill:test", event.Metadata["subject_id"])
	assert.Equal(t, "fs.write", event.Metadata["capability"])
	assert.Equal(t, "/tmp", event.Metadata["scope"])
}

func TestEventType_Values(t *testing.T) {
	// Verify all required event types exist
	requiredTypes := []EventType{
		EventTypeFSWrite,
		EventTypeFSReadDenied,
		EventTypeFSBlockedAlways,
		EventTypePermissionGrant,
		EventTypePermissionRevoke,
		EventTypePermissionDenied,
		EventTypeSandboxBlockedSyscall,
		EventTypeCredentialAccessed,
		EventTypeTaskPlanApproved,
		EventTypeTaskPlanRejected,
		EventTypeGoosedStart,
		EventTypeGoosedStop,
		EventTypeRitualJournalInvoke,
	}

	for _, eventType := range requiredTypes {
		// Verify each type has a valid string representation
		assert.NotEmpty(t, eventType.String(), "EventType %v should have non-empty string representation", eventType)
	}
}

func TestSeverity_Values(t *testing.T) {
	// Verify all severity levels
	severities := []Severity{
		SeverityDebug,
		SeverityInfo,
		SeverityWarning,
		SeverityError,
		SeverityCritical,
	}

	for _, sev := range severities {
		assert.NotEmpty(t, sev.String(), "Severity %v should have non-empty string representation", sev)
	}
}

func TestAuditEvent_WithNilMetadata(t *testing.T) {
	// Arrange: Event with nil metadata
	event := AuditEvent{
		Timestamp: time.Now(),
		Type:      EventTypeFSWrite,
		Severity:  SeverityInfo,
		Message:   "Test",
		Metadata:  nil,
	}

	// Act: Marshal to JSON
	data, err := json.Marshal(event)

	// Assert: Should succeed even with nil metadata
	require.NoError(t, err)

	var unmarshaled map[string]any
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// metadata should be null or empty
	_, hasMetadata := unmarshaled["metadata"]
	assert.False(t, hasMetadata, "nil metadata should not appear in JSON")
}

func TestNewAuditEvent(t *testing.T) {
	// Arrange: Test data
	timestamp := time.Date(2026, 4, 29, 12, 34, 56, 0, time.UTC)
	metadata := map[string]string{
		"path": "/tmp/test.txt",
	}

	// Act: Create new event
	event := NewAuditEvent(timestamp, EventTypeFSWrite, SeverityInfo, "Test message", metadata)

	// Assert: Verify all fields
	assert.Equal(t, timestamp, event.Timestamp)
	assert.Equal(t, EventTypeFSWrite, event.Type)
	assert.Equal(t, SeverityInfo, event.Severity)
	assert.Equal(t, "Test message", event.Message)
	assert.Equal(t, metadata, event.Metadata)
}

func TestEventType_String(t *testing.T) {
	// Test string representations for all event types
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventTypeFSWrite, "fs.write"},
		{EventTypeFSReadDenied, "fs.read.denied"},
		{EventTypeFSBlockedAlways, "fs.blocked_always"},
		{EventTypePermissionGrant, "permission.grant"},
		{EventTypePermissionRevoke, "permission.revoke"},
		{EventTypePermissionDenied, "permission.denied"},
		{EventTypeSandboxBlockedSyscall, "sandbox.blocked_syscall"},
		{EventTypeCredentialAccessed, "credential.accessed"},
		{EventTypeTaskPlanApproved, "task.plan_approved"},
		{EventTypeTaskPlanRejected, "task.plan_rejected"},
		{EventTypeGoosedStart, "goosed.start"},
		{EventTypeGoosedStop, "goosed.stop"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.String())
		})
	}
}

func TestSeverity_String(t *testing.T) {
	// Test string representations for all severity levels
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityDebug, "debug"},
		{SeverityInfo, "info"},
		{SeverityWarning, "warning"},
		{SeverityError, "error"},
		{SeverityCritical, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.severity.String())
		})
	}
}
