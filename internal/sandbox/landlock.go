//go:build linux
// +build linux

// Package sandbox provides Linux Landlock LSM-based sandbox implementation.
// SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
package sandbox

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/modu-ai/goose/internal/audit"
	"github.com/modu-ai/goose/internal/fsaccess"
	"go.uber.org/zap"
)

// LandlockStubActive indicates whether the Landlock sandbox is running in stub mode.
// When true, no kernel-level isolation is active; only the goose-sandbox helper
// binary provides real enforcement. Callers can check this variable programmatically.
//
// @MX:NOTE: [AUTO] Stub mode indicator for runtime detection
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
var LandlockStubActive = false

// landlockSandbox implements Sandbox using Linux Landlock LSM (kernel 5.13+).
//
// REQ-SANDBOX-003: Linux Landlock LSM + Seccomp-BPF filter
// AC-SANDBOX-01: Same blocked_always enforcement across 3 platforms
// AC-SANDBOX-02: Block /proc/self/root/* traversal attempts
//
// @MX:ANCHOR: [AUTO] Linux Landlock sandbox implementation
// @MX:REASON: Primary sandbox implementation for Linux, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
type landlockSandbox struct {
	cfg     Config
	active  bool
	ruleset string // Cached Landlock ruleset
}

// newLandlockSandbox creates a new Linux Landlock sandbox instance.
// @MX:ANCHOR: [AUTO] Landlock sandbox constructor
// @MX:REASON: Factory function called by New(), fan_in >= 3
func newLandlockSandbox(cfg Config) (Sandbox, error) {
	// Check if Landlock is available
	if !isLandlockAvailable() {
		cfg.Logger.Warn("Landlock not available in kernel (requires 5.13+), using fallback")
		if cfg.FallbackBehavior == "refuse" {
			return nil, fmt.Errorf("landlock not available (kernel < 5.13) and fallback_behavior: refuse")
		}
		// Fallback: log warning and return no-op sandbox
		cfg.Logger.Warn("Allowing execution without Landlock sandbox (DANGEROUS)")
		return &noopSandbox{}, nil
	}

	return &landlockSandbox{
		cfg:    cfg,
		active: false,
	}, nil
}

// newSeatbeltSandboxStub is a stub for macOS Seatbelt factory on linux platforms.
// @MX:NOTE: [AUTO] Stub factory function for build compatibility
func newSeatbeltSandbox(cfg Config) (Sandbox, error) {
	return &noopSandbox{}, nil
}

// Activate enables the Linux Landlock sandbox with the given policy.
// It generates Landlock rules from the SecurityPolicy and applies them.
//
// REQ-SANDBOX-003: Use Landlock LSM for FS access rules
// REQ-SANDBOX-003: Seccomp-BPF filter for syscall restriction
// REQ-SANDBOX-005: Blocked syscall → audit.log event + return to parent process
//
// AC-SANDBOX-03: Sandbox failure → refuse fallback
//
// @MX:ANCHOR: [AUTO] Landlock activation function
// @MX:REASON: Converts policy to kernel rules, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003, REQ-SANDBOX-005, AC-SANDBOX-03
func (s *landlockSandbox) Activate(policy *fsaccess.SecurityPolicy) error {
	// Emit runtime warning that this is a stub implementation
	s.cfg.Logger.Warn(
		"Landlock sandbox is a STUB — no kernel-level isolation is active",
		zap.String("warning", "Production deployment requires the goose-sandbox helper binary"),
	)

	// Set stub mode flag
	LandlockStubActive = true

	s.cfg.Logger.Info("Activating Linux Landlock sandbox (stub mode)")

	// Generate Landlock ruleset from security policy
	ruleset, err := s.generateLandlockRuleset(policy)
	if err != nil {
		s.cfg.Logger.Error("Failed to generate Landlock ruleset", zap.Error(err))
		return fmt.Errorf("failed to generate Landlock ruleset: %w", err)
	}

	s.ruleset = ruleset
	s.cfg.Logger.Debug("Generated Landlock ruleset", zap.String("ruleset", ruleset))

	// Validate the ruleset
	if err := s.validateRuleset(ruleset); err != nil {
		s.cfg.Logger.Error("Landlock ruleset validation failed", zap.Error(err))

		// Log to audit log
		_ = s.cfg.AuditWriter.Write(audit.NewAuditEvent(
			time.Now(),
			audit.EventTypeSandboxBlockedSyscall,
			audit.SeverityCritical,
			"Sandbox activation failed: ruleset validation error",
			map[string]string{
				"error": err.Error(),
				"ruleset": ruleset,
			},
		))

		return fmt.Errorf("Landlock ruleset validation failed: %w", err)
	}

	s.active = true

	// Log activation with stub warning to audit log
	_ = s.cfg.AuditWriter.Write(audit.NewAuditEvent(
		time.Now(),
		audit.EventTypeGoosedStart,
		audit.SeverityWarning,
		"Linux Landlock sandbox activated in STUB mode — no kernel-level enforcement",
		map[string]string{
			"ruleset": s.ruleset,
			"stub_mode": "true",
			"warning": "Production deployment requires the goose-sandbox helper binary",
		},
	))

	return nil
}

// Deactivate disables the sandbox.
// Note: Landlock cannot be deactivated once applied (kernel limitation).
func (s *landlockSandbox) Deactivate() error {
	if !s.active {
		return nil
	}
	s.cfg.Logger.Warn("Landlock sandbox cannot be deactivated once applied")
	return fmt.Errorf("landlock sandbox cannot be deactivated once applied")
}

// IsActive returns whether the sandbox is currently active.
func (s *landlockSandbox) IsActive() bool {
	return s.active
}

// generateLandlockRuleset converts a SecurityPolicy to Landlock ruleset format.
//
// REQ-SANDBOX-003: Block write access to ~/.ssh, /etc, /var, /proc, /sys, /dev
// REQ-SANDBOX-003: Block /proc/self/root/* traversal (symlink escape prevention)
// AC-SANDBOX-01: Same blocked_always enforcement across platforms
// AC-SANDBOX-02: Block /proc/self/root/* traversal attempts
//
// @MX:ANCHOR: [AUTO] Landlock ruleset generator
// @MX:REASON: Converts fsaccess policy to Landlock format, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003, AC-SANDBOX-01, AC-SANDBOX-02
func (s *landlockSandbox) generateLandlockRuleset(policy *fsaccess.SecurityPolicy) (string, error) {
	var buf bytes.Buffer

	// Since we can't directly use Landlock syscalls without CGo,
	// we generate a shell script that uses the landlock-restrict tool
	// This is a simplified implementation; production use would require
	// a native Go Landlock wrapper or the goose-sandbox helper binary

	buf.WriteString("#!/bin/bash\n")
	buf.WriteString("# Landlock sandbox ruleset generated by goose\n")
	buf.WriteString("# WARNING: This is a stub implementation.\n")
	buf.WriteString("# Full Landlock enforcement requires the goose-sandbox helper binary.\n")
	buf.WriteString("\n")

	// Block sensitive paths (REQ-SANDBOX-003, AC-SANDBOX-01)
	sensitivePaths := []string{
		"/etc",
		"/var",
		"/proc",
		"/sys",
		"/dev",
	}

	// Add user SSH path
	homeDir, err := os.UserHomeDir()
	if err == nil {
		sensitivePaths = append(sensitivePaths, filepath.Join(homeDir, ".ssh"))
	}

	// Generate rules for blocking sensitive paths
	buf.WriteString("# Blocked paths (read/write denied)\n")
	for _, path := range sensitivePaths {
		// Expand globs and add to ruleset
		buf.WriteString(fmt.Sprintf("echo \"Blocking: %s\"\n", path))
	}

	// Block /proc/self/root/* traversal (AC-SANDBOX-02)
	buf.WriteString("\n# Block /proc/self/root/* traversal (symlink escape)\n")
	buf.WriteString("echo \"Blocking: /proc/*/root/*\"\n")

	// Allow write access to explicitly allowed paths (from policy)
	if len(policy.WritePaths) > 0 {
		buf.WriteString("\n# Allowed write paths\n")
		for _, path := range policy.WritePaths {
			buf.WriteString(fmt.Sprintf("echo \"Allowing write: %s\"\n", path))
		}
	}

	// Allow read access to explicitly allowed paths (from policy)
	if len(policy.ReadPaths) > 0 {
		buf.WriteString("\n# Allowed read paths\n")
		for _, path := range policy.ReadPaths {
			buf.WriteString(fmt.Sprintf("echo \"Allowing read: %s\"\n", path))
		}
	}

	// Block all paths in blocked_always (AC-SANDBOX-01)
	for _, path := range policy.BlockedAlways {
		buf.WriteString(fmt.Sprintf("echo \"Blocking (always): %s\"\n", path))
	}

	// Note: This is a stub. Real Landlock enforcement would require:
	// 1. A native Go Landlock wrapper using raw syscalls
	// 2. Or the goose-sandbox helper binary with setuid capabilities
	// For now, we log the ruleset for manual verification
	buf.WriteString("\n# STUB: Ruleset generated but not enforced\n")
	buf.WriteString("# Real enforcement requires goose-sandbox helper binary\n")

	return buf.String(), nil
}

// validateRuleset checks if the Landlock ruleset is valid.
// Since this is a stub implementation, we do basic validation.
func (s *landlockSandbox) validateRuleset(ruleset string) error {
	// Basic validation: check that ruleset is not empty
	if strings.TrimSpace(ruleset) == "" {
		return fmt.Errorf("landlock ruleset is empty")
	}

	// In a full implementation, this would:
	// 1. Parse the ruleset syntax
	// 2. Check that all referenced paths exist
	// 3. Verify Landlock ABI version
	// 4. Test with a dry-run of the goose-sandbox helper

	s.cfg.Logger.Debug("Landlock ruleset validation passed (stub)")
	return nil
}

// isLandlockAvailable checks if the kernel supports Landlock.
// It reads /proc/sys/kernel/landlock_* to check availability.
func isLandlockAvailable() bool {
	// Check for Landlock support by reading sysfs
	_, err := os.Stat("/proc/sys/kernel/landlock_access_fs")
	if err != nil {
		return false
	}

	// Additional check: verify kernel version is 5.13+
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return false
	}

	// Parse release version (uname.Release is [65]int8 on Linux)
	var releaseBytes []byte
	for _, b := range uname.Release {
		if b == 0 {
			break
		}
		releaseBytes = append(releaseBytes, byte(b))
	}
	release := string(releaseBytes)

	// Simple version check (e.g., "5.13.0-generic")
	parts := strings.Split(release, ".")
	if len(parts) < 2 {
		return false
	}

	major := parts[0]
	minor := parts[1]

	// Check if major > 5 or (major == 5 and minor >= 13)
	if major == "5" && minor >= "13" {
		return true
	}
	if major > "5" {
		return true
	}

	return false
}
