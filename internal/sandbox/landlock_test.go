//go:build linux
// +build linux

// Package sandbox provides tests for Linux Landlock implementation.
// SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
package sandbox

import (
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/fsaccess"
	"go.uber.org/zap/zaptest"
)

// TestLandlockSandbox tests the Linux Landlock sandbox implementation.
// @MX:TEST: [AUTO] Landlock sandbox test
func TestLandlockSandbox(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	// Test with Landlock available (if kernel supports it)
	sb, err := newLandlockSandbox(cfg)
	if err != nil {
		// Landlock not available, skip test
		if strings.Contains(err.Error(), "landlock not available") && cfg.FallbackBehavior == "refuse" {
			t.Skip("Landlock not available on this kernel (requires 5.13+)")
		}
		t.Fatalf("newLandlockSandbox() error = %v", err)
	}

	// If we got a noopSandbox, Landlock is not available
	if _, ok := sb.(*noopSandbox); ok {
		t.Skip("Landlock not available on this kernel (requires 5.13+)")
	}

	landlock, ok := sb.(*landlockSandbox)
	if !ok {
		t.Fatal("newLandlockSandbox() did not return *landlockSandbox")
	}

	// Test initial state
	if landlock.IsActive() {
		t.Error("IsActive() = true, want false before activation")
	}

	// Test Activate with a sample policy
	policy := &fsaccess.SecurityPolicy{
		WritePaths:    []string{"/tmp/goose"},
		ReadPaths:     []string{"/home/user"},
		BlockedAlways: []string{"/etc/passwd"},
	}

	if err := landlock.Activate(policy); err != nil {
		t.Errorf("Activate() error = %v", err)
	}

	if !landlock.IsActive() {
		t.Error("IsActive() = false, want true after activation")
	}

	// Test that ruleset was generated
	if landlock.ruleset == "" {
		t.Error("GenerateLandlockRuleset() produced empty ruleset")
	}
}

// TestGenerateLandlockRuleset tests the Landlock ruleset generation logic.
// REQ-SANDBOX-003: Block write access to ~/.ssh, /etc, /var, /proc, /sys, /dev
// AC-SANDBOX-01: Same blocked_always enforcement across platforms
// AC-SANDBOX-02: Block /proc/self/root/* traversal attempts
// @MX:TEST: [AUTO] Landlock ruleset generation test
func TestGenerateLandlockRuleset(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newLandlockSandbox(cfg)
	if err != nil {
		// Landlock not available, skip test
		if strings.Contains(err.Error(), "landlock not available") && cfg.FallbackBehavior == "refuse" {
			t.Skip("Landlock not available on this kernel (requires 5.13+)")
		}
		t.Fatalf("newLandlockSandbox() error = %v", err)
	}

	// If we got a noopSandbox, Landlock is not available
	if _, ok := sb.(*noopSandbox); ok {
		t.Skip("Landlock not available on this kernel (requires 5.13+)")
	}

	landlock := sb.(*landlockSandbox)

	policy := &fsaccess.SecurityPolicy{
		WritePaths:    []string{"/tmp/goose", "/var/tmp/goose"},
		ReadPaths:     []string{"/home/user", "/usr/share"},
		BlockedAlways: []string{"/etc/shadow", "/root/.ssh"},
	}

	ruleset, err := landlock.generateLandlockRuleset(policy)
	if err != nil {
		t.Fatalf("generateLandlockRuleset() error = %v", err)
	}

	// Verify ruleset is not empty
	if strings.TrimSpace(ruleset) == "" {
		t.Error("Landlock ruleset is empty")
	}

	// Verify ruleset is a shell script
	if !strings.Contains(ruleset, "#!/bin/bash") {
		t.Error("Landlock ruleset missing shebang")
	}

	// Verify ruleset contains blocking rules for sensitive paths (REQ-SANDBOX-003)
	sensitivePaths := []string{"/etc", "/var", "/proc", "/sys", "/dev"}
	for _, path := range sensitivePaths {
		if !strings.Contains(ruleset, "Blocking: "+path) {
			t.Errorf("Landlock ruleset missing block for sensitive path %s", path)
		}
	}

	// Verify ruleset blocks /proc/self/root/* traversal (AC-SANDBOX-02)
	if !strings.Contains(ruleset, "/proc/*/root/*") {
		t.Error("Landlock ruleset missing /proc/*/root/* traversal block")
	}

	// Verify ruleset includes allowed write paths
	if !strings.Contains(ruleset, "Allowing write: /tmp/goose") {
		t.Error("Landlock ruleset missing allowed write path /tmp/goose")
	}

	// Verify ruleset includes allowed read paths
	if !strings.Contains(ruleset, "Allowing read: /home/user") {
		t.Error("Landlock ruleset missing allowed read path /home/user")
	}

	// Verify ruleset includes blocked_always paths (AC-SANDBOX-01)
	if !strings.Contains(ruleset, "Blocking (always): /etc/shadow") {
		t.Error("Landlock ruleset missing blocked_always path /etc/shadow")
	}

	// Verify ruleset contains stub warning
	if !strings.Contains(ruleset, "STUB") {
		t.Error("Landlock ruleset missing stub implementation warning")
	}
}

// TestLandlockDeactivate tests that Deactivate returns an error.
// @MX:TEST: [AUTO] Landlock deactivate test
func TestLandlockDeactivate(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newLandlockSandbox(cfg)
	if err != nil {
		// Landlock not available, skip test
		if strings.Contains(err.Error(), "landlock not available") && cfg.FallbackBehavior == "refuse" {
			t.Skip("Landlock not available on this kernel (requires 5.13+)")
		}
		t.Fatalf("newLandlockSandbox() error = %v", err)
	}

	// If we got a noopSandbox, Landlock is not available
	if _, ok := sb.(*noopSandbox); ok {
		t.Skip("Landlock not available on this kernel (requires 5.13+)")
	}

	landlock := sb.(*landlockSandbox)

	// Deactivate before activation should succeed (no-op)
	if err := landlock.Deactivate(); err != nil {
		t.Errorf("Deactivate() before activation error = %v, want nil", err)
	}

	// Activate sandbox
	policy := &fsaccess.SecurityPolicy{}
	if err := landlock.Activate(policy); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	// Deactivate after activation should fail (Landlock limitation)
	if err := landlock.Deactivate(); err == nil {
		t.Error("Deactivate() after activation error = nil, want error (Landlock cannot deactivate)")
	}
}

// TestValidateRuleset tests the Landlock ruleset validation.
// @MX:TEST: [AUTO] Landlock ruleset validation test
func TestValidateRuleset(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newLandlockSandbox(cfg)
	if err != nil {
		// Landlock not available, skip test
		if strings.Contains(err.Error(), "landlock not available") && cfg.FallbackBehavior == "refuse" {
			t.Skip("Landlock not available on this kernel (requires 5.13+)")
		}
		t.Fatalf("newLandlockSandbox() error = %v", err)
	}

	// If we got a noopSandbox, Landlock is not available
	if _, ok := sb.(*noopSandbox); ok {
		t.Skip("Landlock not available on this kernel (requires 5.13+)")
	}

	landlock := sb.(*landlockSandbox)

	// Test valid ruleset
	validRuleset := "#!/bin/bash\necho 'test'\n"
	if err := landlock.validateRuleset(validRuleset); err != nil {
		t.Errorf("validateRuleset() with valid ruleset error = %v, want nil", err)
	}

	// Test empty ruleset
	emptyRuleset := ""
	if err := landlock.validateRuleset(emptyRuleset); err == nil {
		t.Error("validateRuleset() with empty ruleset error = nil, want error")
	}
}

// TestIsLandlockAvailable tests the Landlock availability detection.
// @MX:TEST: [AUTO] Landlock availability detection test
func TestIsLandlockAvailable(t *testing.T) {
	// This test may pass or fail depending on the kernel version
	// We just log the result for informational purposes
	available := isLandlockAvailable()
	t.Logf("Landlock available: %v", available)

	if available {
		t.Log("This kernel supports Landlock (5.13+)")
	} else {
		t.Log("This kernel does not support Landlock (requires 5.13+)")
	}
}

// TestLandlockStubActive tests that the stub mode flag is set correctly.
// @MX:TEST: [AUTO] Landlock stub mode flag test
func TestLandlockStubActive(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newLandlockSandbox(cfg)
	if err != nil {
		// Landlock not available, skip test
		if strings.Contains(err.Error(), "landlock not available") && cfg.FallbackBehavior == "refuse" {
			t.Skip("Landlock not available on this kernel (requires 5.13+)")
		}
		t.Fatalf("newLandlockSandbox() error = %v", err)
	}

	// If we got a noopSandbox, Landlock is not available
	if _, ok := sb.(*noopSandbox); ok {
		t.Skip("Landlock not available on this kernel (requires 5.13+)")
	}

	landlock := sb.(*landlockSandbox)

	// Before activation, stub flag should be false
	if LandlockStubActive {
		t.Error("LandlockStubActive = true before activation, want false")
	}

	// Activate sandbox
	policy := &fsaccess.SecurityPolicy{}
	if err := landlock.Activate(policy); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	// After activation, stub flag should be true (this is a stub implementation)
	if !LandlockStubActive {
		t.Error("LandlockStubActive = false after activation, want true (stub mode)")
	}

	// Reset flag for other tests
	LandlockStubActive = false
}
