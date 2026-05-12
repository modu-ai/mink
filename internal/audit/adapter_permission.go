package audit

import (
	"fmt"

	"github.com/modu-ai/mink/internal/permission"
)

// PermissionAuditor adapts the audit system to implement the permission.Auditor interface.
// It converts PermissionEvent to AuditEvent and writes them to the audit log.
//
// @MX:ANCHOR: [AUTO] Permission event audit adapter
// @MX:REASON: Bridges permission system to audit logging, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-001
type PermissionAuditor struct {
	writer Writer
}

// Writer is the interface for writing audit events.
// This allows mocking in tests and supports multiple implementations.
type Writer interface {
	Write(event AuditEvent) error
	Close() error
}

// NewPermissionAuditor creates a new PermissionAuditor that writes to the given Writer.
func NewPermissionAuditor(writer Writer) *PermissionAuditor {
	return &PermissionAuditor{
		writer: writer,
	}
}

// Record logs a permission event to the audit log.
// It converts the permission.PermissionEvent to an AuditEvent with appropriate
// event type and severity.
//
// REQ-AUDIT-001: 보안 관련 이벤트를 JSON line 형식으로 audit.log에 append
func (a *PermissionAuditor) Record(pEvent permission.PermissionEvent) error {
	// Convert permission event type to audit event type
	eventType, severity := a.convertEventType(pEvent.Type)

	// Build metadata
	metadata := a.buildMetadata(pEvent)

	// Create audit event
	event := NewAuditEvent(
		pEvent.Timestamp,
		eventType,
		severity,
		a.buildMessage(pEvent),
		metadata,
	)

	// Write to audit log
	if err := a.writer.Write(event); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

// convertEventType converts permission event type to audit event type and severity.
func (a *PermissionAuditor) convertEventType(pType string) (EventType, Severity) {
	switch pType {
	case "grant_created", "grant_reused":
		return EventTypePermissionGrant, SeverityInfo
	case "grant_denied":
		return EventTypePermissionDenied, SeverityWarning
	case "grant_revoked":
		return EventTypePermissionRevoke, SeverityWarning
	default:
		// Fallback for unknown types
		return EventTypePermissionDenied, SeverityInfo
	}
}

// buildMetadata creates metadata map from permission event.
func (a *PermissionAuditor) buildMetadata(pEvent permission.PermissionEvent) map[string]string {
	metadata := make(map[string]string)

	if pEvent.SubjectID != "" {
		metadata["subject_id"] = pEvent.SubjectID
	}
	if pEvent.Scope != "" {
		metadata["scope"] = pEvent.Scope
	}
	if pEvent.Reason != "" {
		metadata["reason"] = pEvent.Reason
	}
	if pEvent.InheritedFrom != "" {
		metadata["inherited_from"] = pEvent.InheritedFrom
	}
	// Add capability as string using the String() method
	metadata["capability"] = pEvent.Capability.String()

	return metadata
}

// buildMessage creates a human-readable message from permission event.
func (a *PermissionAuditor) buildMessage(pEvent permission.PermissionEvent) string {
	switch pEvent.Type {
	case "grant_created":
		return fmt.Sprintf("Permission granted to %s for %s", pEvent.SubjectID, pEvent.Capability.String())
	case "grant_reused":
		return fmt.Sprintf("Existing permission reused by %s for %s", pEvent.SubjectID, pEvent.Capability.String())
	case "grant_denied":
		return fmt.Sprintf("Permission denied for %s: %s", pEvent.SubjectID, pEvent.Reason)
	case "grant_revoked":
		return fmt.Sprintf("Permission revoked for %s", pEvent.SubjectID)
	default:
		return fmt.Sprintf("Permission event: %s", pEvent.Type)
	}
}

// Close closes the underlying writer.
func (a *PermissionAuditor) Close() error {
	if a.writer != nil {
		return a.writer.Close()
	}
	return nil
}

// MockWriter is a test double for Writer interface.
type MockWriter struct {
	Events []AuditEvent
	closed bool
}

// NewMockWriter creates a new MockWriter.
func NewMockWriter() *MockWriter {
	return &MockWriter{
		Events: make([]AuditEvent, 0),
	}
}

// Write appends the event to the Events slice.
func (m *MockWriter) Write(event AuditEvent) error {
	m.Events = append(m.Events, event)
	return nil
}

// Close marks the writer as closed.
func (m *MockWriter) Close() error {
	m.closed = true
	return nil
}

// IsClosed returns true if Close was called.
func (m *MockWriter) IsClosed() bool {
	return m.closed
}

// EventCount returns the number of events written.
func (m *MockWriter) EventCount() int {
	return len(m.Events)
}
