// Package fsaccess provides filesystem access control with policy-based security.
// SPEC-GOOSE-FS-ACCESS-001
package fsaccess

import (
	"context"
	"fmt"
	"time"

	"github.com/modu-ai/mink/internal/audit"
)

// AuditLogger logs filesystem access decisions to the audit log.
// REQ-FSACCESS-005: Log (operation, path, allowed, reason, at) to audit
// AC-05: Audit log completeness
//
// @MX:ANCHOR: [AUTO] Audit logger interface
// @MX:REASON: Used by all FS access operations, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-005, AC-05
type AuditLogger struct {
	writer audit.Writer
}

// NewAuditLogger creates a new AuditLogger that writes to the given Writer.
// The writer is typically a FileWriter or DualWriter from the audit package.
//
// @MX:ANCHOR: [AUTO] AuditLogger constructor
// @MX:REASON: Creates audit logger instance, fan_in >= 3
func NewAuditLogger(writer audit.Writer) *AuditLogger {
	return &AuditLogger{
		writer: writer,
	}
}

// LogDecision logs a filesystem access decision to the audit log.
// The log entry includes all required fields per AC-05:
// - operation: the FS operation type (read/write/create/delete)
// - path: the filesystem path being accessed
// - allowed: the access decision (allow/deny/ask)
// - reason: explanation for the decision
// - at: timestamp of the decision
//
// REQ-FSACCESS-005: Log every FS access decision
// AC-05: Complete audit trail with all required fields
//
// @MX:ANCHOR: [AUTO] Core audit logging function
// @MX:REASON: Called for every FS access decision, fan_in >= 5
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-005, AC-05
func (l *AuditLogger) LogDecision(ctx context.Context, path string, operation Operation, result AccessResult) error {
	// Determine event type and severity based on decision
	var eventType audit.EventType
	var severity audit.Severity

	switch result.Decision {
	case DecisionAllow:
		// Allowed operations are informational
		eventType = audit.EventTypeFSWrite
		severity = audit.SeverityInfo
	case DecisionDeny:
		// Denied operations are warnings (blocked by policy)
		if result.Policy == "blocked_always" {
			eventType = audit.EventTypeFSBlockedAlways
			severity = audit.SeverityCritical
		} else {
			eventType = audit.EventTypeFSReadDenied
			severity = audit.SeverityWarning
		}
	case DecisionAsk:
		// Ask decisions are informational (pending user input)
		eventType = audit.EventTypeFSReadDenied
		severity = audit.SeverityInfo
	default:
		eventType = audit.EventTypeFSWrite
		severity = audit.SeverityInfo
	}

	// Adjust event type based on operation for read operations
	if operation == OperationRead && result.Decision == DecisionAllow {
		// Read operations use a different event type when allowed
		// Note: We don't have a specific "fs.read" event type, so use generic write
		eventType = audit.EventTypeFSWrite
	}

	// Create metadata map with all required fields (AC-05)
	metadata := map[string]string{
		"operation": operation.String(),
		"path":      path,
		"allowed":   result.Decision.String(),
		"reason":    result.Reason,
		"policy":    result.Policy,
	}

	// Create audit event
	event := audit.NewAuditEvent(
		time.Now(),
		eventType,
		severity,
		fmt.Sprintf("FS access %s: %s %s", result.Decision.String(), operation.String(), path),
		metadata,
	)

	// Write to audit log
	return l.writer.Write(event)
}

// AskFunc is a callback function type for requesting user permission.
// It is used when the DecisionEngine returns DecisionAsk.
//
// The function should prompt the user and return true if allowed, false if denied.
// It should return an error if the prompt fails or is cancelled.
//
// REQ-FSACCESS-001: AskUserQuestion fallback for unmatched paths
//
// @MX:ANCHOR: [AUTO] User permission callback type
// @MX:REASON: Defines interface for user interaction, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-FS-ACCESS-001 REQ-FSACCESS-001
type AskFunc func(ctx context.Context, path string, operation Operation) (bool, error)
