//go:build !darwin && !linux
// +build !darwin,!linux

// Package sandbox provides stub implementation for unsupported platforms.
// SPEC-GOOSE-SECURITY-SANDBOX-001
package sandbox

import (
	"fmt"
	"runtime"

	"github.com/modu-ai/mink/internal/audit"
	"go.uber.org/zap"
)

// newSeatbeltSandboxStub is a stub for macOS Seatbelt factory on non-darwin platforms.
// @MX:NOTE: [AUTO] Stub factory function for build compatibility
func newSeatbeltSandbox(cfg Config) (Sandbox, error) {
	return newDefaultSandbox(cfg)
}

// newLandlockSandboxStub is a stub for Linux Landlock factory on non-linux platforms.
// @MX:NOTE: [AUTO] Stub factory function for build compatibility
func newLandlockSandbox(cfg Config) (Sandbox, error) {
	return newDefaultSandbox(cfg)
}

// defaultSandbox is a stub implementation for unsupported platforms.
// It logs warnings and either refuses or allows execution based on fallback behavior.
//
// REQ-SANDBOX-004: Sandbox failure → refuse to run (fallback_behavior: refuse)
// AC-SANDBOX-03: Sandbox failure → refuse fallback
//
// @MX:NOTE: [AUTO] Stub sandbox for unsupported platforms
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-004, AC-SANDBOX-03
type defaultSandbox struct {
	cfg    Config
	active bool
}

// newDefaultSandbox creates a new default sandbox for unsupported platforms.
// @MX:ANCHOR: [AUTO] Default sandbox constructor
// @MX:REASON: Factory function for unsupported platforms, fan_in >= 3
func newDefaultSandbox(cfg Config) (Sandbox, error) {
	cfg.Logger.Warn(fmt.Sprintf("Sandbox not supported on %s", runtime.GOOS))

	// If fallback behavior is "refuse", return error immediately
	if cfg.FallbackBehavior == "refuse" {
		return nil, fmt.Errorf("sandbox not supported on %s (fallback_behavior: refuse)", runtime.GOOS)
	}

	// Log warning but allow execution
	cfg.Logger.Warn(fmt.Sprintf("Allowing execution without sandbox on %s (DANGEROUS)", runtime.GOOS))
	return &defaultSandbox{
		cfg:    cfg,
		active: false,
	}, nil
}

// Activate logs a warning and either returns error or succeeds based on fallback behavior.
// REQ-SANDBOX-004: fallback_behavior "refuse" → refuse to run
func (s *defaultSandbox) Activate(policy *fsaccess.SecurityPolicy) error {
	s.cfg.Logger.Warn(fmt.Sprintf("Sandbox activation not supported on %s", runtime.GOOS))

	if s.cfg.FallbackBehavior == "refuse" {
		return fmt.Errorf("sandbox not supported on %s (fallback_behavior: refuse)", runtime.GOOS)
	}

	// Fallback: log warning but allow execution
	s.cfg.Logger.Warn("Allowing execution without sandbox (DANGEROUS)")
	return nil
}

// Deactivate is a no-op for the default sandbox.
func (s *defaultSandbox) Deactivate() error {
	return nil
}

// IsActive always returns false for the default sandbox.
func (s *defaultSandbox) IsActive() bool {
	return false
}
