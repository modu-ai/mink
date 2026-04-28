package audit

import (
	"fmt"
	"os"
	"path/filepath"
)

// DualWriter writes audit events to two locations simultaneously:
// 1. Global audit log (e.g., ~/.goose/logs/audit.log)
// 2. Project-local audit log (e.g., ./.goose/logs/audit.local.log)
//
// REQ-AUDIT-003: WHEN 프로젝트 레벨 이벤트가 발생할 때,
//
//	the system SHALL ./.goose/logs/audit.local.log에도 복제
//	하여 프로젝트 소유자가 검토 가능하게 한다
//
// @MX:ANCHOR: [AUTO] Dual-location audit writer
// @MX:REASON: Core component for multi-location audit logging
// @MX:SPEC: SPEC-GOOSE-AUDIT-001 REQ-AUDIT-003
type DualWriter struct {
	globalWriter *RotatingWriter
	localWriter  *RotatingWriter
	enabled      bool
}

// DualWriterConfig configures a DualWriter.
type DualWriterConfig struct {
	// GlobalPath is the path to the global audit log (e.g., ~/.goose/logs/audit.log)
	GlobalPath string
	// LocalPath is the path to the project-local audit log (e.g., ./.goose/logs/audit.local.log)
	LocalPath string
	// MaxSize is the maximum size before rotation (default: 100MB)
	MaxSize int64
	// EnableLocal enables writing to the local audit log
	EnableLocal bool
}

// NewDualWriter creates a new DualWriter with the specified configuration.
// If local writing is disabled or the local path is empty, only global logging is performed.
func NewDualWriter(config DualWriterConfig) (*DualWriter, error) {
	// Create global writer (always enabled)
	globalWriter, err := NewRotatingWriter(config.GlobalPath, config.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create global writer: %w", err)
	}

	var localWriter *RotatingWriter
	enabled := false

	// Create local writer if enabled
	if config.EnableLocal && config.LocalPath != "" {
		localWriter, err = NewRotatingWriter(config.LocalPath, config.MaxSize)
		if err != nil {
			// If local writer creation fails, log warning but don't fail
			// Project-local logging is optional
			globalWriter.Close()
			return nil, fmt.Errorf("failed to create local writer: %w", err)
		}
		enabled = true
	}

	return &DualWriter{
		globalWriter: globalWriter,
		localWriter:  localWriter,
		enabled:      enabled,
	}, nil
}

// Write writes an audit event to both global and local logs.
// If local writing is disabled, only the global log is written.
// If either write fails, the error is returned but the other write may have succeeded.
func (w *DualWriter) Write(event AuditEvent) error {
	// Always write to global log
	if err := w.globalWriter.Write(event); err != nil {
		return fmt.Errorf("failed to write to global log: %w", err)
	}

	// Write to local log if enabled
	if w.enabled && w.localWriter != nil {
		if err := w.localWriter.Write(event); err != nil {
			// Local write failure is non-fatal
			// Log to global as a warning event in production
			return fmt.Errorf("failed to write to local log: %w", err)
		}
	}

	return nil
}

// Close closes both the global and local writers.
func (w *DualWriter) Close() error {
	var errs []error

	if w.globalWriter != nil {
		if err := w.globalWriter.Close(); err != nil {
			errs = append(errs, fmt.Errorf("global writer close: %w", err))
		}
	}

	if w.localWriter != nil {
		if err := w.localWriter.Close(); err != nil {
			errs = append(errs, fmt.Errorf("local writer close: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// GlobalPath returns the path to the global audit log.
func (w *DualWriter) GlobalPath() string {
	if w == nil || w.globalWriter == nil {
		return ""
	}
	return w.globalWriter.Path()
}

// LocalPath returns the path to the local audit log.
func (w *DualWriter) LocalPath() string {
	if w == nil || w.localWriter == nil {
		return ""
	}
	return w.localWriter.Path()
}

// IsLocalEnabled returns true if local logging is enabled.
func (w *DualWriter) IsLocalEnabled() bool {
	return w != nil && w.enabled
}

// DefaultGlobalAuditPath returns the default global audit log path.
// It uses the GOOSE_HOME environment variable or defaults to ~/.goose/logs/audit.log
func DefaultGlobalAuditPath() (string, error) {
	// Check GOOSE_HOME env var
	gooseHome := os.Getenv("GOOSE_HOME")
	if gooseHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to determine home directory: %w", err)
		}
		gooseHome = filepath.Join(homeDir, ".goose")
	}

	return filepath.Join(gooseHome, "logs", "audit.log"), nil
}

// DefaultLocalAuditPath returns the default project-local audit log path.
// It returns ./.goose/logs/audit.local.log
func DefaultLocalAuditPath() string {
	return filepath.Join(".goose", "logs", "audit.local.log")
}
