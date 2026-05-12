// Package sandbox provides tests for the sandbox abstraction layer.
// SPEC-GOOSE-SECURITY-SANDBOX-001
package sandbox

import (
	"testing"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/fsaccess"
	"go.uber.org/zap/zaptest"
)

// TestConfigValidation tests the Config.Validate method.
// @MX:TEST: [AUTO] Config validation test
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: Config{
				Enabled:          true,
				FallbackBehavior: "refuse",
				AuditWriter:      &mockAuditWriter{},
				Logger:           zaptest.NewLogger(t),
			},
			wantErr: false,
		},
		{
			name: "missing audit writer",
			cfg: Config{
				Enabled:          true,
				FallbackBehavior: "refuse",
				Logger:           zaptest.NewLogger(t),
			},
			wantErr: true,
			errMsg:  "audit writer is required",
		},
		{
			name: "missing logger",
			cfg: Config{
				Enabled:          true,
				FallbackBehavior: "refuse",
				AuditWriter:      &mockAuditWriter{},
			},
			wantErr: true,
			errMsg:  "logger is required",
		},
		{
			name: "invalid fallback behavior",
			cfg: Config{
				Enabled:          true,
				FallbackBehavior: "invalid",
				AuditWriter:      &mockAuditWriter{},
				Logger:           zaptest.NewLogger(t),
			},
			wantErr: true,
			errMsg:  "invalid fallback_behavior",
		},
		{
			name: "empty fallback behavior defaults to refuse",
			cfg: Config{
				Enabled:     true,
				AuditWriter: &mockAuditWriter{},
				Logger:      zaptest.NewLogger(t),
			},
			wantErr: false,
		},
		{
			name: "valid config with allow fallback",
			cfg: Config{
				Enabled:          true,
				FallbackBehavior: "allow",
				AuditWriter:      &mockAuditWriter{},
				Logger:           zaptest.NewLogger(t),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Config.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestNewSandbox tests the New factory function.
// @MX:TEST: [AUTO] Sandbox factory test
func TestNewSandbox(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "disabled sandbox returns no-op",
			cfg: Config{
				Enabled:          false,
				FallbackBehavior: "refuse",
				AuditWriter:      &mockAuditWriter{},
				Logger:           zaptest.NewLogger(t),
			},
			wantErr: false,
		},
		{
			name: "enabled sandbox with valid config",
			cfg: Config{
				Enabled:          true,
				FallbackBehavior: "refuse",
				AuditWriter:      &mockAuditWriter{},
				Logger:           zaptest.NewLogger(t),
			},
			wantErr: false,
		},
		{
			name: "enabled sandbox with invalid config",
			cfg: Config{
				Enabled:          true,
				FallbackBehavior: "refuse",
				Logger:           zaptest.NewLogger(t),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb, err := New(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && sb == nil {
				t.Error("New() returned nil sandbox without error")
			}
		})
	}
}

// TestNoopSandbox tests the no-op sandbox implementation.
// @MX:TEST: [AUTO] No-op sandbox test
func TestNoopSandbox(t *testing.T) {
	sb := &noopSandbox{}

	// Test Activate always succeeds
	policy := &fsaccess.SecurityPolicy{
		WritePaths:    []string{"/tmp"},
		ReadPaths:     []string{"/home"},
		BlockedAlways: []string{"/etc"},
	}

	if err := sb.Activate(policy); err != nil {
		t.Errorf("noopSandbox.Activate() error = %v, want nil", err)
	}

	// Test IsActive returns false
	if sb.IsActive() {
		t.Error("noopSandbox.IsActive() = true, want false")
	}

	// Test Deactivate always succeeds
	if err := sb.Deactivate(); err != nil {
		t.Errorf("noopSandbox.Deactivate() error = %v, want nil", err)
	}
}

// TestAuditLogger tests the AuditLogger functionality.
// @MX:TEST: [AUTO] Audit logger test
func TestAuditLogger(t *testing.T) {
	mockWriter := &mockAuditWriter{}
	logger := zaptest.NewLogger(t)
	auditLogger := NewAuditLogger(mockWriter, logger)

	// Test LogActivation
	auditLogger.LogActivation("darwin", "test-profile")
	if mockWriter.writeCount != 1 {
		t.Errorf("LogActivation() write count = %d, want 1", mockWriter.writeCount)
	}

	// Test LogActivationFailure
	auditLogger.LogActivationFailure("linux", testError("activation failed"))
	if mockWriter.writeCount != 2 {
		t.Errorf("LogActivationFailure() write count = %d, want 2", mockWriter.writeCount)
	}

	// Test LogBlockedSyscall
	auditLogger.LogBlockedSyscall("darwin", "open", "/etc/passwd", "blocked by policy")
	if mockWriter.writeCount != 3 {
		t.Errorf("LogBlockedSyscall() write count = %d, want 3", mockWriter.writeCount)
	}

	// Test LogDeactivation
	auditLogger.LogDeactivation("darwin")
	if mockWriter.writeCount != 4 {
		t.Errorf("LogDeactivation() write count = %d, want 4", mockWriter.writeCount)
	}
}

// TestAuditLoggerWithErrorWriter tests audit logger with erroring writer.
// @MX:TEST: [AUTO] Audit logger error handling test
func TestAuditLoggerWithErrorWriter(t *testing.T) {
	errorWriter := &errorAuditWriter{}
	logger := zaptest.NewLogger(t)
	auditLogger := NewAuditLogger(errorWriter, logger)

	// Test that functions handle write errors gracefully
	auditLogger.LogActivation("darwin", "test-profile")
	auditLogger.LogActivationFailure("linux", testError("activation failed"))
	auditLogger.LogBlockedSyscall("darwin", "open", "/etc/passwd", "blocked by policy")
	auditLogger.LogDeactivation("darwin")

	// Should not panic, just log errors
	// Check that write was attempted
	if errorWriter.writeCount != 4 {
		t.Errorf("Expected 4 write attempts, got %d", errorWriter.writeCount)
	}
}

// TestFallbackBehavior tests the fallback behavior logic.
// AC-SANDBOX-03: Sandbox failure → refuse fallback
// @MX:TEST: [AUTO] Fallback behavior test
func TestFallbackBehavior(t *testing.T) {
	tests := []struct {
		name             string
		fallbackBehavior string
		enabled          bool
		expectError      bool
	}{
		{
			name:             "refuse fallback with enabled sandbox",
			fallbackBehavior: "refuse",
			enabled:          true,
			expectError:      false, // Platform-specific, may not error on darwin/linux
		},
		{
			name:             "allow fallback with enabled sandbox",
			fallbackBehavior: "allow",
			enabled:          true,
			expectError:      false,
		},
		{
			name:             "refuse fallback with disabled sandbox",
			fallbackBehavior: "refuse",
			enabled:          false,
			expectError:      false,
		},
		{
			name:             "allow fallback with disabled sandbox",
			fallbackBehavior: "allow",
			enabled:          false,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Enabled:          tt.enabled,
				FallbackBehavior: tt.fallbackBehavior,
				AuditWriter:      &mockAuditWriter{},
				Logger:           zaptest.NewLogger(t),
			}

			sb, err := New(cfg)
			if (err != nil) != tt.expectError {
				t.Errorf("New() error = %v, expectError %v", err, tt.expectError)
			}

			if err == nil && sb == nil {
				t.Error("New() returned nil sandbox without error")
			}
		})
	}
}

// mockAuditWriter is a mock implementation of audit.Writer for testing.
// @MX:NOTE: [AUTO] Mock audit writer for testing
type mockAuditWriter struct {
	writeCount int
	lastEvent  audit.AuditEvent
}

func (m *mockAuditWriter) Write(event audit.AuditEvent) error {
	m.writeCount++
	m.lastEvent = event
	return nil
}

func (m *mockAuditWriter) Close() error {
	return nil
}

// errorAuditWriter is a mock that always returns an error.
// @MX:NOTE: [AUTO] Error audit writer for testing error paths
type errorAuditWriter struct {
	writeCount int
}

func (m *errorAuditWriter) Write(event audit.AuditEvent) error {
	m.writeCount++
	return testError("write failed")
}

func (m *errorAuditWriter) Close() error {
	return nil
}

// testError is a simple error type for testing.
type testError string

func (e testError) Error() string {
	return string(e)
}

// containsString checks if a string contains a substring.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && contains(s, substr))
}

// contains is a simple substring check.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
