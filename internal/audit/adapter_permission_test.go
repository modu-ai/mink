package audit

import (
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Capability constants from permission package
const (
	capFSWrite = 2 // CapFSWrite
	capFSRead  = 1 // CapFSRead
)

func TestPermissionAuditor_Record_GrantCreated(t *testing.T) {
	// Arrange: Create auditor with mock writer
	mockWriter := NewMockWriter()
	auditor := NewPermissionAuditor(mockWriter)

	pEvent := permission.PermissionEvent{
		Type:       "grant_created",
		SubjectID:  "skill:test-skill",
		Capability: capFSWrite,
		Scope:      "/tmp",
		Timestamp:  time.Now().UTC(),
	}

	// Act: Record event
	err := auditor.Record(pEvent)

	// Assert: Event should be written
	require.NoError(t, err)
	assert.Equal(t, 1, mockWriter.EventCount())

	event := mockWriter.Events[0]
	assert.Equal(t, EventTypePermissionGrant, event.Type)
	assert.Equal(t, SeverityInfo, event.Severity)
	assert.Equal(t, "skill:test-skill", event.Metadata["subject_id"])
	assert.Equal(t, "/tmp", event.Metadata["scope"])
}

func TestPermissionAuditor_Record_GrantDenied(t *testing.T) {
	// Arrange
	mockWriter := NewMockWriter()
	auditor := NewPermissionAuditor(mockWriter)

	pEvent := permission.PermissionEvent{
		Type:       "grant_denied",
		SubjectID:  "skill:test-skill",
		Capability: capFSWrite,
		Scope:      "/etc",
		Reason:     "Security policy violation",
		Timestamp:  time.Now().UTC(),
	}

	// Act
	err := auditor.Record(pEvent)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, mockWriter.EventCount())

	event := mockWriter.Events[0]
	assert.Equal(t, EventTypePermissionDenied, event.Type)
	assert.Equal(t, SeverityWarning, event.Severity)
	assert.Equal(t, "Security policy violation", event.Metadata["reason"])
}

func TestPermissionAuditor_Record_GrantRevoked(t *testing.T) {
	// Arrange
	mockWriter := NewMockWriter()
	auditor := NewPermissionAuditor(mockWriter)

	pEvent := permission.PermissionEvent{
		Type:       "grant_revoked",
		SubjectID:  "skill:test-skill",
		Capability: capFSWrite,
		Scope:      "/tmp",
		Timestamp:  time.Now().UTC(),
	}

	// Act
	err := auditor.Record(pEvent)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, mockWriter.EventCount())

	event := mockWriter.Events[0]
	assert.Equal(t, EventTypePermissionRevoke, event.Type)
	assert.Equal(t, SeverityWarning, event.Severity)
}

func TestPermissionAuditor_Record_WithInheritance(t *testing.T) {
	// Arrange
	mockWriter := NewMockWriter()
	auditor := NewPermissionAuditor(mockWriter)

	pEvent := permission.PermissionEvent{
		Type:          "grant_reused",
		SubjectID:     "agent:sub-agent",
		Capability:    capFSRead,
		Scope:         "/tmp",
		InheritedFrom: "skill:parent-skill",
		Timestamp:     time.Now().UTC(),
	}

	// Act
	err := auditor.Record(pEvent)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, mockWriter.EventCount())

	event := mockWriter.Events[0]
	assert.Equal(t, "skill:parent-skill", event.Metadata["inherited_from"])
}

func TestPermissionAuditor_Close(t *testing.T) {
	// Arrange
	mockWriter := NewMockWriter()
	auditor := NewPermissionAuditor(mockWriter)

	// Act
	err := auditor.Close()

	// Assert
	require.NoError(t, err)
	assert.True(t, mockWriter.IsClosed())
}

func TestPermissionAuditor_BuildMessage(t *testing.T) {
	// Arrange
	tests := []struct {
		name     string
		pType    string
		subject  string
		cap      permission.Capability
		expected string
	}{
		{
			name:     "grant_created",
			pType:    "grant_created",
			subject:  "skill:test",
			cap:      2, // CapFSWrite
			expected: "Permission granted to skill:test for fs_write",
		},
		{
			name:     "grant_denied",
			pType:    "grant_denied",
			subject:  "agent:malicious",
			cap:      0, // CapNet
			expected: "Permission denied for agent:malicious: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock writer for each test
			mockWriter := NewMockWriter()
			auditor := NewPermissionAuditor(mockWriter)

			pEvent := permission.PermissionEvent{
				Type:       tt.pType,
				SubjectID:  tt.subject,
				Capability: tt.cap,
				Timestamp:  time.Now().UTC(),
			}

			err := auditor.Record(pEvent)
			require.NoError(t, err)

			assert.Contains(t, mockWriter.Events[0].Message, tt.expected)
		})
	}
}
