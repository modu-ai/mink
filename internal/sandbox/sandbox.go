// Package sandbox provides OS-level filesystem and process isolation.
// SPEC-GOOSE-SECURITY-SANDBOX-001
package sandbox

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/fsaccess"
	"go.uber.org/zap"
)

// Sandbox provides OS-level filesystem and process isolation.
// It converts high-level security policies into platform-specific kernel rules.
//
// REQ-SANDBOX-001: Convert security.yaml policy to kernel rules on activation
// REQ-SANDBOX-004: Sandbox failure → refuse to run (fallback_behavior: refuse)
//
// @MX:ANCHOR: [AUTO] Core sandbox interface for platform isolation
// @MX:REASON: Central abstraction for all sandbox implementations, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-001, REQ-SANDBOX-004
type Sandbox interface {
	// Activate enables the sandbox with the given policy.
	// This converts the high-level SecurityPolicy into platform-specific kernel rules.
	// Returns an error if sandbox activation fails (see Config.FallbackBehavior).
	Activate(policy *fsaccess.SecurityPolicy) error

	// Deactivate disables the sandbox (if supported by the platform).
	// Some platforms (like macOS Seatbelt) cannot deactivate mid-session.
	// Returns an error if deactivation is not supported or fails.
	Deactivate() error

	// IsActive returns whether the sandbox is currently active.
	IsActive() bool
}

// Config controls sandbox behavior and fallback policy.
//
// REQ-SANDBOX-004: fallback_behavior "refuse" (default) or "allow"
//
// @MX:ANCHOR: [AUTO] Sandbox configuration structure
// @MX:REASON: Used by New and all sandbox implementations, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-004
type Config struct {
	// Enabled enables or disables sandbox functionality.
	// When false, New() returns a no-op sandbox that always succeeds.
	Enabled bool

	// FallbackBehavior defines what happens when sandbox activation fails.
	// "refuse" (default): Return error and refuse to run (REQ-SANDBOX-004)
	// "allow": Log warning but allow execution without sandbox (DANGEROUS)
	FallbackBehavior string

	// AuditWriter is the audit log writer for security events.
	// Required for logging blocked syscalls and activation failures.
	AuditWriter audit.Writer

	// Logger is the structured logger for debug and warning messages.
	Logger *zap.Logger

	// TimeFunc returns the current time for audit logging.
	// Defaults to time.Now if not set.
	// @MX:NOTE: [AUTO] Time function for audit logging
	TimeFunc func() time.Time
}

// Validate checks if the Config is valid and returns an error if not.
// This ensures required fields are set before creating a Sandbox.
func (c *Config) Validate() error {
	if c.AuditWriter == nil {
		return errors.New("audit writer is required")
	}
	if c.Logger == nil {
		return errors.New("logger is required")
	}
	if c.FallbackBehavior == "" {
		c.FallbackBehavior = "refuse" // Default to secure behavior
	}
	if c.FallbackBehavior != "refuse" && c.FallbackBehavior != "allow" {
		return fmt.Errorf("invalid fallback_behavior: %s (must be 'refuse' or 'allow')", c.FallbackBehavior)
	}
	if c.TimeFunc == nil {
		c.TimeFunc = time.Now // Default to time.Now
	}
	return nil
}

// New creates a platform-appropriate Sandbox based on the current OS.
// Returns a no-op sandbox if Config.Enabled is false.
// Returns an error if Config validation fails or platform is unsupported.
//
// REQ-SANDBOX-002: macOS Seatbelt profile blocking ~/.ssh, /etc, /var, /proc
// REQ-SANDBOX-003: Linux Landlock LSM + Seccomp-BPF filter
// AC-SANDBOX-05: Force non-root execution
//
// @MX:ANCHOR: [AUTO] Sandbox factory function
// @MX:REASON: Creates appropriate sandbox implementation, fan_in >= 5
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-002, REQ-SANDBOX-003, REQ-SANDBOX-005
func New(cfg Config) (Sandbox, error) {
	// Validate config
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Return no-op sandbox if disabled
	if !cfg.Enabled {
		cfg.Logger.Info("Sandbox disabled, returning no-op implementation")
		return &noopSandbox{}, nil
	}

	// Check for root execution (AC-SANDBOX-05)
	// This is a basic check; platform-specific implementations may have additional checks
	// TODO: Add proper root detection based on platform

	// Create platform-specific implementation
	switch runtime.GOOS {
	case "darwin":
		cfg.Logger.Info("Creating macOS Seatbelt sandbox")
		return newSeatbeltSandbox(cfg)
	case "linux":
		cfg.Logger.Info("Creating Linux Landlock sandbox")
		return newLandlockSandbox(cfg)
	default:
		// Unsupported platform
		if cfg.FallbackBehavior == "refuse" {
			return nil, fmt.Errorf("sandbox not supported on %s (fallback_behavior: refuse)", runtime.GOOS)
		}
		// Fallback: log warning and return no-op sandbox
		cfg.Logger.Warn(fmt.Sprintf("Sandbox not supported on %s, allowing execution without sandbox (DANGEROUS)", runtime.GOOS))
		return &noopSandbox{}, nil
	}
}

// noopSandbox is a no-op implementation for when sandbox is disabled or unsupported.
// @MX:NOTE: [AUTO] No-op sandbox for disabled/unsupported platforms
type noopSandbox struct{}

// Activate always succeeds for no-op sandbox.
func (s *noopSandbox) Activate(policy *fsaccess.SecurityPolicy) error {
	return nil
}

// Deactivate always succeeds for no-op sandbox.
func (s *noopSandbox) Deactivate() error {
	return nil
}

// IsActive always returns false for no-op sandbox.
func (s *noopSandbox) IsActive() bool {
	return false
}
