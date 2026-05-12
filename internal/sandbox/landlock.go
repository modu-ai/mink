//go:build linux

// Package sandbox provides Linux Landlock LSM-based sandbox implementation.
// SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/modu-ai/mink/internal/audit"
	"github.com/modu-ai/mink/internal/fsaccess"
	"go.uber.org/zap"
)

// LandlockStubActive indicates whether the Landlock sandbox is running in stub mode.
// When true, no kernel-level isolation is active; the sandbox is a no-op.
// This flag is set to false when real Landlock enforcement is active.
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
	cfg        Config
	active     bool
	abiVersion int
}

// newLandlockSandbox creates a new Linux Landlock sandbox instance.
// @MX:ANCHOR: [AUTO] Landlock sandbox constructor
// @MX:REASON: Factory function called by New(), fan_in >= 3
func newLandlockSandbox(cfg Config) (Sandbox, error) {
	// Check if Landlock is available
	abiVersion := getLandlockAbiVersion()
	if abiVersion == 0 {
		cfg.Logger.Warn("Landlock not available in kernel (requires 5.13+), using fallback")
		if cfg.FallbackBehavior == "refuse" {
			return nil, fmt.Errorf("landlock not available (kernel < 5.13) and fallback_behavior: refuse")
		}
		// Fallback: log warning and return no-op sandbox
		cfg.Logger.Warn("Allowing execution without Landlock sandbox (DANGEROUS)")
		return &noopSandbox{}, nil
	}

	abiName := "ABI v1"
	if abiVersion == LandlockAbiVersion2 {
		abiName = "ABI v2"
	}

	cfg.Logger.Info(fmt.Sprintf("Landlock available (%s)", abiName))

	return &landlockSandbox{
		cfg:        cfg,
		active:     false,
		abiVersion: abiVersion,
	}, nil
}

// newSeatbeltSandboxStub is a stub for macOS Seatbelt factory on linux platforms.
// @MX:NOTE: [AUTO] Stub factory function for build compatibility
func newSeatbeltSandbox(cfg Config) (Sandbox, error) {
	return &noopSandbox{}, nil
}

// Activate enables the Linux Landlock sandbox with the given policy.
// It creates a Landlock ruleset, adds path rules, and enforces sandboxing.
//
// REQ-SANDBOX-003: Use Landlock LSM for FS access rules
// REQ-SANDBOX-005: Blocked syscall → audit.log event + return to parent process
//
// AC-SANDBOX-03: Sandbox failure → refuse fallback
//
// @MX:ANCHOR: [AUTO] Landlock activation function
// @MX:REASON: Converts policy to kernel rules and enforces them, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003, REQ-SANDBOX-005, AC-SANDBOX-03
func (s *landlockSandbox) Activate(policy *fsaccess.SecurityPolicy) error {
	s.cfg.Logger.Info("Activating Linux Landlock sandbox (real kernel enforcement)")

	// Determine which access rights to handle based on ABI version
	var handledAccessFS uint64
	if s.abiVersion >= LandlockAbiVersion2 {
		// ABI v2: include all rights including accessFSRefer
		handledAccessFS = accessFSExecute |
			accessFSWriteFile |
			accessFSReadFile |
			accessFSReadDir |
			accessFSRemoveDir |
			accessFSRemoveFile |
			accessFSMakeChar |
			accessFSMakeDir |
			accessFSMakeReg |
			accessFSMakeSock |
			accessFSMakeFifo |
			accessFSMakeBlock |
			accessFSMakeSym |
			accessFSTruncate |
			accessFSRefer
	} else {
		// ABI v1: exclude accessFSRefer
		handledAccessFS = accessFSExecute |
			accessFSWriteFile |
			accessFSReadFile |
			accessFSReadDir |
			accessFSRemoveDir |
			accessFSRemoveFile |
			accessFSMakeChar |
			accessFSMakeDir |
			accessFSMakeReg |
			accessFSMakeSock |
			accessFSMakeFifo |
			accessFSMakeBlock |
			accessFSMakeSym |
			accessFSTruncate
	}

	// Create the Landlock ruleset
	s.cfg.Logger.Debug("Creating Landlock ruleset", zap.Int("abi_version", s.abiVersion))
	attr := &landlockRulesetAttr{
		HandledAccessFS: handledAccessFS,
	}

	rulesetFd, err := landlockCreateRuleset(attr, unsafeSizeofLandlockRulesetAttr, 0)
	if err != nil {
		s.cfg.Logger.Error("Failed to create Landlock ruleset", zap.Error(err))

		// Log to audit log
		_ = s.cfg.AuditWriter.Write(audit.NewAuditEvent(
			s.cfg.TimeFunc(),
			audit.EventTypeSandboxBlockedSyscall,
			audit.SeverityCritical,
			"Sandbox activation failed: ruleset creation error",
			map[string]string{
				"error":       err.Error(),
				"abi_version": fmt.Sprintf("%d", s.abiVersion),
			},
		))

		return fmt.Errorf("failed to create Landlock ruleset: %w", err)
	}
	defer syscall.Close(rulesetFd)

	// Add allowed paths from policy
	// Landlock uses a whitelist approach: only explicitly allowed paths are accessible

	// Add read paths
	for _, path := range policy.ReadPaths {
		if err := s.addReadPathRule(rulesetFd, path); err != nil {
			s.cfg.Logger.Warn("Failed to add read path rule",
				zap.String("path", path),
				zap.Error(err))
			// Continue with other paths; optional paths may not exist
		}
	}

	// Add write paths
	for _, path := range policy.WritePaths {
		if err := s.addWritePathRule(rulesetFd, path); err != nil {
			s.cfg.Logger.Warn("Failed to add write path rule",
				zap.String("path", path),
				zap.Error(err))
			// Continue with other paths; optional paths may not exist
		}
	}

	// Add default safe paths (required for basic process operation)
	// These are paths that almost all processes need access to
	defaultPaths := []string{
		"/dev/null",    // For null device access
		"/dev/zero",    // For zero device access
		"/dev/urandom", // For random number generation
		"/tmp",         // For temporary files
		"/var/tmp",     // For temporary files
	}

	for _, path := range defaultPaths {
		if err := s.addReadPathRule(rulesetFd, path); err != nil {
			s.cfg.Logger.Debug("Failed to add default path rule",
				zap.String("path", path),
				zap.Error(err))
			// Default paths are optional; continue if they don't exist
		}
	}

	// Apply the ruleset to the current process
	s.cfg.Logger.Debug("Enforcing Landlock ruleset on current process")
	if err := landlockRestrictSelf(rulesetFd); err != nil {
		s.cfg.Logger.Error("Failed to enforce Landlock ruleset", zap.Error(err))

		// Log to audit log
		_ = s.cfg.AuditWriter.Write(audit.NewAuditEvent(
			s.cfg.TimeFunc(),
			audit.EventTypeSandboxBlockedSyscall,
			audit.SeverityCritical,
			"Sandbox activation failed: enforcement error",
			map[string]string{
				"error":       err.Error(),
				"abi_version": fmt.Sprintf("%d", s.abiVersion),
			},
		))

		return fmt.Errorf("failed to enforce Landlock ruleset: %w", err)
	}

	// Mark sandbox as active and disable stub mode
	s.active = true
	LandlockStubActive = false

	// Log successful activation to audit log
	_ = s.cfg.AuditWriter.Write(audit.NewAuditEvent(
		s.cfg.TimeFunc(),
		audit.EventTypeGoosedStart,
		audit.SeverityInfo,
		"Linux Landlock sandbox activated with real kernel-level enforcement",
		map[string]string{
			"abi_version":    fmt.Sprintf("%d", s.abiVersion),
			"read_paths":     fmt.Sprintf("%d", len(policy.ReadPaths)),
			"write_paths":    fmt.Sprintf("%d", len(policy.WritePaths)),
			"blocked_always": fmt.Sprintf("%d", len(policy.BlockedAlways)),
			"enforcement":    "kernel",
			"stub_mode":      "false",
		},
	))

	s.cfg.Logger.Info("Landlock sandbox activated successfully",
		zap.Int("abi_version", s.abiVersion),
		zap.Int("read_paths", len(policy.ReadPaths)),
		zap.Int("write_paths", len(policy.WritePaths)),
	)

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

// addReadPathRule adds a read-only rule for a path.
// @MX:ANCHOR: [AUTO] Add read-only path rule to Landlock ruleset
// @MX:REASON: Reusable function for adding read path rules, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func (s *landlockSandbox) addReadPathRule(rulesetFd int, path string) error {
	// Resolve the path to an absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}

	// Check if path exists before adding rule
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path %s does not exist", absPath)
		}
		return fmt.Errorf("failed to stat path %s: %w", absPath, err)
	}

	s.cfg.Logger.Debug("Adding read path rule", zap.String("path", absPath))

	// Add the path rule with read-only access
	if err := addPathRule(rulesetFd, absPath, false, s.abiVersion); err != nil {
		return err
	}

	return nil
}

// addWritePathRule adds a read-write rule for a path.
// @MX:ANCHOR: [AUTO] Add read-write path rule to Landlock ruleset
// @MX:REASON: Reusable function for adding write path rules, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func (s *landlockSandbox) addWritePathRule(rulesetFd int, path string) error {
	// Resolve the path to an absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
	}

	// Check if path exists before adding rule
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("path %s does not exist", absPath)
		}
		return fmt.Errorf("failed to stat path %s: %w", absPath, err)
	}

	s.cfg.Logger.Debug("Adding write path rule", zap.String("path", absPath))

	// Add the path rule with read-write access
	if err := addPathRule(rulesetFd, absPath, true, s.abiVersion); err != nil {
		return err
	}

	return nil
}

// isLandlockAvailable checks if the kernel supports Landlock.
// It reads /proc/sys/kernel/landlock_* to check availability.
//
// @MX:ANCHOR: [AUTO] Landlock availability detection
// @MX:REASON: Required for runtime capability detection, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func isLandlockAvailable() bool {
	abiVersion := getLandlockAbiVersion()
	return abiVersion > 0
}

// getKernelVersion extracts the kernel version from uname.
// Returns major, minor, and release version as integers.
//
// @MX:ANCHOR: [AUTO] Extract kernel version from uname
// @MX:REASON: Required for Landlock availability check, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func getKernelVersion() (major int, minor int, err error) {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return 0, 0, fmt.Errorf("failed to get uname: %w", err)
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

	// Parse version (e.g., "5.13.0-generic")
	parts := strings.Split(release, ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid kernel version format: %s", release)
	}

	// Parse major and minor version numbers
	majorStr := parts[0]
	minorStr := parts[1]

	// Remove non-digit characters from minor version (e.g., "13-generic" -> "13")
	for i, c := range minorStr {
		if c < '0' || c > '9' {
			minorStr = minorStr[:i]
			break
		}
	}

	// Convert to integers
	if _, err := fmt.Sscanf(majorStr, "%d", &major); err != nil {
		return 0, 0, fmt.Errorf("failed to parse major version: %w", err)
	}
	if _, err := fmt.Sscanf(minorStr, "%d", &minor); err != nil {
		return 0, 0, fmt.Errorf("failed to parse minor version: %w", err)
	}

	return major, minor, nil
}
