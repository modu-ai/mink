// Package sandbox provides audit logging integration for sandbox events.
// SPEC-GOOSE-SECURITY-SANDBOX-001
package sandbox

import (
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"go.uber.org/zap"
)

// AuditLogger handles sandbox-specific audit logging.
// It wraps the generic audit.Writer and provides sandbox-specific event logging.
//
// REQ-SANDBOX-005: Blocked syscall → audit.log event + return to parent process
//
// @MX:ANCHOR: [AUTO] Sandbox audit logger
// @MX:REASON: Central audit logging for all sandbox implementations, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-005
type AuditLogger struct {
	writer audit.Writer
	logger *zap.Logger
}

// NewAuditLogger creates a new sandbox audit logger.
// @MX:ANCHOR: [AUTO] Audit logger constructor
// @MX:REASON: Factory function used by all sandbox implementations, fan_in >= 3
func NewAuditLogger(writer audit.Writer, logger *zap.Logger) *AuditLogger {
	return &AuditLogger{
		writer: writer,
		logger: logger,
	}
}

// LogActivation logs a successful sandbox activation event.
// REQ-SANDBOX-001: When goosed starts, activate platform sandbox
func (a *AuditLogger) LogActivation(platform string, profile string) {
	event := audit.NewAuditEvent(
		time.Now(),
		audit.EventTypeGoosedStart,
		audit.SeverityInfo,
		"Sandbox activated successfully",
		map[string]string{
			"platform": platform,
			"profile":  profile,
		},
	)

	if err := a.writer.Write(event); err != nil {
		a.logger.Error("Failed to write sandbox activation audit log", zap.Error(err))
	}
}

// LogActivationFailure logs a sandbox activation failure event.
// REQ-SANDBOX-004: Sandbox failure → refuse to run
func (a *AuditLogger) LogActivationFailure(platform string, err error) {
	event := audit.NewAuditEvent(
		time.Now(),
		audit.EventTypeSandboxBlockedSyscall,
		audit.SeverityCritical,
		"Sandbox activation failed",
		map[string]string{
			"platform": platform,
			"error":    err.Error(),
		},
	)

	if writeErr := a.writer.Write(event); writeErr != nil {
		a.logger.Error("Failed to write sandbox activation failure audit log",
			zap.Error(err),
			zap.Error(writeErr))
	}
}

// LogBlockedSyscall logs a blocked syscall event.
// REQ-SANDBOX-005: Blocked syscall → audit.log event + return to parent process
func (a *AuditLogger) LogBlockedSyscall(platform string, syscall string, path string, reason string) {
	event := audit.NewAuditEvent(
		time.Now(),
		audit.EventTypeSandboxBlockedSyscall,
		audit.SeverityWarning,
		"Sandbox blocked syscall",
		map[string]string{
			"platform": platform,
			"syscall":  syscall,
			"path":     path,
			"reason":   reason,
		},
	)

	if err := a.writer.Write(event); err != nil {
		a.logger.Error("Failed to write blocked syscall audit log", zap.Error(err))
	}
}

// LogDeactivation logs a sandbox deactivation event (if supported).
func (a *AuditLogger) LogDeactivation(platform string) {
	event := audit.NewAuditEvent(
		time.Now(),
		audit.EventTypeGoosedStop,
		audit.SeverityInfo,
		"Sandbox deactivated",
		map[string]string{
			"platform": platform,
		},
	)

	if err := a.writer.Write(event); err != nil {
		a.logger.Error("Failed to write sandbox deactivation audit log", zap.Error(err))
	}
}
