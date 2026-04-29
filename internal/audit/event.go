// Package audit provides append-only audit logging for security events.
// SPEC-GOOSE-AUDIT-001
package audit

import (
	"encoding/json"
	"strings"
	"time"
)

// EventType represents the type of audit event.
// REQ-AUDIT-001: Event types for security-related operations
type EventType string

const (
	// EventTypeFSWrite is emitted when a file write operation occurs
	EventTypeFSWrite EventType = "fs.write"
	// EventTypeFSReadDenied is emitted when a file read is denied
	EventTypeFSReadDenied EventType = "fs.read.denied"
	// EventTypeFSBlockedAlways is emitted when a blocked_always path is accessed
	EventTypeFSBlockedAlways EventType = "fs.blocked_always"
	// EventTypePermissionGrant is emitted when a permission is granted
	EventTypePermissionGrant EventType = "permission.grant"
	// EventTypePermissionRevoke is emitted when a permission is revoked
	EventTypePermissionRevoke EventType = "permission.revoke"
	// EventTypePermissionDenied is emitted when a permission request is denied
	EventTypePermissionDenied EventType = "permission.denied"
	// EventTypeSandboxBlockedSyscall is emitted when sandbox blocks a syscall
	EventTypeSandboxBlockedSyscall EventType = "sandbox.blocked_syscall"
	// EventTypeCredentialAccessed is emitted when a credential is accessed (key reference only, not value)
	EventTypeCredentialAccessed EventType = "credential.accessed"
	// EventTypeTaskPlanApproved is emitted when a task plan is approved
	EventTypeTaskPlanApproved EventType = "task.plan_approved"
	// EventTypeTaskPlanRejected is emitted when a task plan is rejected
	EventTypeTaskPlanRejected EventType = "task.plan_rejected"
	// EventTypeGoosedStart is emitted when the goosed daemon starts
	EventTypeGoosedStart EventType = "goosed.start"
	// EventTypeGoosedStop is emitted when the goosed daemon stops
	EventTypeGoosedStop EventType = "goosed.stop"
)

// String returns the string representation of the event type
func (e EventType) String() string {
	return string(e)
}

// Severity represents the severity level of an audit event
type Severity string

const (
	// SeverityDebug is for debugging information
	SeverityDebug Severity = "debug"
	// SeverityInfo is for informational events
	SeverityInfo Severity = "info"
	// SeverityWarning is for warning events
	SeverityWarning Severity = "warning"
	// SeverityError is for error events
	SeverityError Severity = "error"
	// SeverityCritical is for critical security events
	SeverityCritical Severity = "critical"
)

// String returns the string representation of the severity level
func (s Severity) String() string {
	return string(s)
}

// sanitizeMessage removes control characters that could enable log injection.
// Replaces newlines, carriage returns, and other control chars with escaped equivalents.
// This prevents attackers from injecting fake log entries or breaking log parsing.
//
// @MX:ANCHOR: [AUTO] Message sanitization for all audit events
// @MX:REASON: Called by NewAuditEvent, used by all event creation paths, fan_in >= 5
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 security requirement for log injection prevention
func sanitizeMessage(msg string) string {
	// Replace common control characters with escaped representations
	// This prevents log injection attacks while preserving readability
	replacer := strings.NewReplacer(
		"\n", "\\n",
		"\r", "\\r",
		"\t", "\\t",
		"\x00", "\\x00",
		"\x1b", "\\x1b", // ESC character
	)
	return replacer.Replace(msg)
}

// AuditEvent represents a single audit log entry.
// AC-AUDIT-01: JSON line format for all events
//
// @MX:ANCHOR: [AUTO] Core event structure for all audit logging
// @MX:REASON: Every audit write operation uses this structure, fan_in >= 5
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-001
type AuditEvent struct {
	// Timestamp is when the event occurred (RFC3339)
	Timestamp time.Time `json:"timestamp"`
	// Type is the event category (e.g., "fs.write", "permission.grant")
	Type EventType `json:"type"`
	// Severity is the event severity level
	Severity Severity `json:"severity"`
	// Message is a human-readable description
	Message string `json:"message"`
	// Metadata contains additional event-specific key-value pairs
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NewAuditEvent creates a new AuditEvent with the given fields.
// This is a convenience constructor to ensure all required fields are set.
// Message field is sanitized to prevent log injection attacks.
func NewAuditEvent(timestamp time.Time, eventType EventType, severity Severity, message string, metadata map[string]string) AuditEvent {
	return AuditEvent{
		Timestamp: timestamp,
		Type:      eventType,
		Severity:  severity,
		Message:   sanitizeMessage(message),
		Metadata:  metadata,
	}
}

// MarshalJSON implements custom JSON marshaling for AuditEvent.
// This ensures consistent JSON output format across the codebase.
func (e AuditEvent) MarshalJSON() ([]byte, error) {
	// Use anonymous struct to control JSON field order and handle metadata
	type jsonAuditEvent struct {
		Timestamp string            `json:"timestamp"`
		Type      EventType         `json:"type"`
		Severity  Severity          `json:"severity"`
		Message   string            `json:"message"`
		Metadata  map[string]string `json:"metadata,omitempty"`
	}

	return json.Marshal(jsonAuditEvent{
		Timestamp: e.Timestamp.UTC().Format(time.RFC3339),
		Type:      e.Type,
		Severity:  e.Severity,
		Message:   e.Message,
		Metadata:  e.Metadata,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for AuditEvent.
// This allows reading audit events from log files.
func (e *AuditEvent) UnmarshalJSON(data []byte) error {
	// Use anonymous struct to parse JSON
	type jsonAuditEvent struct {
		Timestamp string            `json:"timestamp"`
		Type      EventType         `json:"type"`
		Severity  Severity          `json:"severity"`
		Message   string            `json:"message"`
		Metadata  map[string]string `json:"metadata,omitempty"`
	}

	var tmp jsonAuditEvent
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	// Parse timestamp
	timestamp, err := time.Parse(time.RFC3339, tmp.Timestamp)
	if err != nil {
		return err
	}

	e.Timestamp = timestamp
	e.Type = tmp.Type
	e.Severity = tmp.Severity
	e.Message = tmp.Message
	e.Metadata = tmp.Metadata

	return nil
}
