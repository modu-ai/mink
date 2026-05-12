//go:build darwin
// +build darwin

// Package sandbox provides tests for macOS Seatbelt implementation.
// SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-002
package sandbox

import (
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/fsaccess"
	"go.uber.org/zap/zaptest"
)

// TestSeatbeltSandbox tests the macOS Seatbelt sandbox implementation.
// @MX:TEST: [AUTO] Seatbelt sandbox test
func TestSeatbeltSandbox(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newSeatbeltSandbox(cfg)
	if err != nil {
		t.Fatalf("newSeatbeltSandbox() error = %v", err)
	}

	seatbelt, ok := sb.(*seatbeltSandbox)
	if !ok {
		t.Fatal("newSeatbeltSandbox() did not return *seatbeltSandbox")
	}

	// Test initial state
	if seatbelt.IsActive() {
		t.Error("IsActive() = true, want false before activation")
	}

	// Test Activate with a sample policy
	policy := &fsaccess.SecurityPolicy{
		WritePaths:    []string{"/tmp/goose"},
		ReadPaths:     []string{"/home/user"},
		BlockedAlways: []string{"/etc/passwd"},
	}

	if err := seatbelt.Activate(policy); err != nil {
		t.Errorf("Activate() error = %v", err)
	}

	if !seatbelt.IsActive() {
		t.Error("IsActive() = false, want true after activation")
	}

	// Test that profile was generated
	if seatbelt.profile == "" {
		t.Error("GenerateSBPLProfile() produced empty profile")
	}
}

// TestGenerateSBPLProfile tests the SBPL profile generation logic.
// REQ-SANDBOX-002: Block write access to ~/.ssh, /etc, /var, /proc, /sys, /dev
// AC-SANDBOX-01: Same blocked_always enforcement across platforms
// AC-SANDBOX-02: Block /proc/self/root/* traversal attempts
// @MX:TEST: [AUTO] SBPL profile generation test
func TestGenerateSBPLProfile(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newSeatbeltSandbox(cfg)
	if err != nil {
		t.Fatalf("newSeatbeltSandbox() error = %v", err)
	}

	seatbelt := sb.(*seatbeltSandbox)

	policy := &fsaccess.SecurityPolicy{
		WritePaths:    []string{"/tmp/goose", "/var/tmp/goose"},
		ReadPaths:     []string{"/home/user", "/usr/share"},
		BlockedAlways: []string{"/etc/shadow", "/root/.ssh"},
	}

	profile, err := seatbelt.generateSBPLProfile(policy)
	if err != nil {
		t.Fatalf("generateSBPLProfile() error = %v", err)
	}

	// Verify profile contains version declaration
	if !strings.Contains(profile, "(version 1)") {
		t.Error("SBPL profile missing version declaration")
	}

	// Verify profile denies all by default
	if !strings.Contains(profile, "(deny default)") {
		t.Error("SBPL profile missing default deny rule")
	}

	// Verify profile blocks sensitive paths (REQ-SANDBOX-002)
	sensitivePaths := []string{"/etc", "/var", "/proc", "/sys", "/dev"}
	for _, path := range sensitivePaths {
		if !strings.Contains(profile, "(subpath \""+path+"\")") {
			t.Errorf("SBPL profile missing block for sensitive path %s", path)
		}
	}

	// Verify profile blocks /proc/self/root/* traversal (AC-SANDBOX-02)
	if !strings.Contains(profile, "(regex #\"^/proc/[^/]+/root/\"#)") {
		t.Error("SBPL profile missing /proc/*/root/* traversal block")
	}

	// Verify profile blocks network by default
	if !strings.Contains(profile, "(deny network*)") {
		t.Error("SBPL profile missing network deny rule")
	}

	// Verify profile allows localhost network
	if !strings.Contains(profile, "(allow network* (local unix))") {
		t.Error("SBPL profile missing localhost network allow rule")
	}

	// Verify profile includes allowed write paths
	if !strings.Contains(profile, "(allow file-write*") {
		t.Error("SBPL profile missing file-write allow section")
	}
	if !strings.Contains(profile, "(subpath \"/tmp/goose\")") {
		t.Error("SBPL profile missing allowed write path /tmp/goose")
	}

	// Verify profile includes allowed read paths
	if !strings.Contains(profile, "(allow file-read*") {
		t.Error("SBPL profile missing file-read allow section")
	}
	if !strings.Contains(profile, "(subpath \"/home/user\")") {
		t.Error("SBPL profile missing allowed read path /home/user")
	}

	// Verify profile includes blocked_always paths (AC-SANDBOX-01)
	if !strings.Contains(profile, "/etc/shadow") {
		t.Error("SBPL profile missing blocked_always path /etc/shadow")
	}
}

// TestSeatbeltDeactivate tests that Deactivate returns an error.
// @MX:TEST: [AUTO] Seatbelt deactivate test
func TestSeatbeltDeactivate(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newSeatbeltSandbox(cfg)
	if err != nil {
		t.Fatalf("newSeatbeltSandbox() error = %v", err)
	}

	seatbelt := sb.(*seatbeltSandbox)

	// Deactivate before activation should succeed (no-op)
	if err := seatbelt.Deactivate(); err != nil {
		t.Errorf("Deactivate() before activation error = %v, want nil", err)
	}

	// Activate sandbox
	policy := &fsaccess.SecurityPolicy{}
	if err := seatbelt.Activate(policy); err != nil {
		t.Fatalf("Activate() error = %v", err)
	}

	// Deactivate after activation should fail (seatbelt limitation)
	if err := seatbelt.Deactivate(); err == nil {
		t.Error("Deactivate() after activation error = nil, want error (seatbelt cannot deactivate)")
	}
}

// TestValidateProfile tests the SBPL profile validation.
// @MX:TEST: [AUTO] SBPL profile validation test
func TestValidateProfile(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newSeatbeltSandbox(cfg)
	if err != nil {
		t.Fatalf("newSeatbeltSandbox() error = %v", err)
	}

	seatbelt := sb.(*seatbeltSandbox)

	// Test valid profile
	validProfile := "(version 1)\n(deny default)\n(allow file-read*)\n"
	if err := seatbelt.validateProfile(validProfile); err != nil {
		t.Errorf("validateProfile() with valid profile error = %v, want nil", err)
	}

	// Test invalid profile (syntax error)
	invalidProfile := "(version 1\n(deny default\n" // Missing closing parenthesis
	if err := seatbelt.validateProfile(invalidProfile); err == nil {
		t.Error("validateProfile() with invalid profile error = nil, want error")
	}
}

// TestGetHomeDir tests the home directory detection.
// @MX:TEST: [AUTO] Home directory detection test
func TestGetHomeDir(t *testing.T) {
	cfg := Config{
		Enabled:          true,
		FallbackBehavior: "refuse",
		AuditWriter:      &mockAuditWriter{},
		Logger:           zaptest.NewLogger(t),
	}

	sb, err := newSeatbeltSandbox(cfg)
	if err != nil {
		t.Fatalf("newSeatbeltSandbox() error = %v", err)
	}

	seatbelt := sb.(*seatbeltSandbox)

	home, err := seatbelt.getHomeDir()
	if err != nil {
		t.Errorf("getHomeDir() error = %v", err)
	}

	if home == "" {
		t.Error("getHomeDir() returned empty string")
	}

	// Verify home path looks valid (should contain user or root)
	if !strings.Contains(home, "Users") && !strings.Contains(home, "home") && !strings.Contains(home, "root") {
		t.Logf("Warning: getHomeDir() returned unexpected path: %s", home)
	}
}
