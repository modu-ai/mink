//go:build linux

// Package sandbox provides Linux Landlock LSM enforcement functions.
// SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
package sandbox

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// oPath is the Linux O_PATH flag for obtaining a file descriptor that refers
// to the filesystem object. Value 0x200000 is arch-independent on Linux.
// Defined as a constant because syscall.O_PATH is not available on macOS.
const oPath = 0x200000

// landlockCreateRuleset creates a new Landlock ruleset.
// It returns a file descriptor for the ruleset, or an error if the syscall fails.
//
// @MX:ANCHOR: [AUTO] Landlock ruleset creation syscall wrapper
// @MX:REASON: Core syscall for creating Landlock rulesets, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func landlockCreateRuleset(attr *landlockRulesetAttr, size uintptr, flags uint) (int, error) {
	r, _, errno := syscall.Syscall(
		sysLandlockCreateRuleset,
		uintptr(unsafe.Pointer(attr)),
		size,
		uintptr(flags),
	)
	if errno != 0 {
		return -1, fmt.Errorf("landlock_create_ruleset: %w", errno)
	}
	return int(r), nil
}

// landlockAddPathRule adds a path-based rule to an existing Landlock ruleset.
// The rule specifies which access rights are allowed for a given directory.
//
// @MX:ANCHOR: [AUTO] Landlock path rule addition syscall wrapper
// @MX:REASON: Core syscall for adding path rules to rulesets, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func landlockAddPathRule(rulesetFd int, ruleType uintptr, attr *landlockPathBeneathAttr) error {
	r, _, errno := syscall.Syscall(
		sysLandlockAddRule,
		uintptr(rulesetFd),
		ruleType,
		uintptr(unsafe.Pointer(attr)),
	)
	if errno != 0 {
		return fmt.Errorf("landlock_add_rule: %w", errno)
	}
	if r != 0 {
		return fmt.Errorf("landlock_add_rule: unexpected return value %d", r)
	}
	return nil
}

// landlockRestrictSelf applies a Landlock ruleset to the current process.
// After this call, the current process (and all future children) will be
// restricted by the ruleset. This is a one-way operation - it cannot be undone.
//
// @MX:ANCHOR: [AUTO] Landlock self-restriction syscall wrapper
// @MX:REASON: Core syscall for enforcing ruleset on current process, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func landlockRestrictSelf(rulesetFd int) error {
	r, _, errno := syscall.Syscall(
		sysLandlockRestrictSelf,
		uintptr(rulesetFd),
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %w", errno)
	}
	if r != 0 {
		return fmt.Errorf("landlock_restrict_self: unexpected return value %d", r)
	}
	return nil
}

// openPathFd opens a file descriptor for a directory with O_PATH | O_CLOEXEC flags.
// This is required for Landlock path-beneath rules.
//
// @MX:ANCHOR: [AUTO] Open directory file descriptor for Landlock rules
// @MX:REASON: Required for adding path rules to Landlock rulesets, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func openPathFd(path string) (int, error) {
	// O_PATH: Obtain a file descriptor without actually opening the file
	// O_CLOEXEC: Set the close-on-exec flag to prevent leaking to child processes
	fd, err := syscall.Open(path, oPath|syscall.O_CLOEXEC, 0)
	if err != nil {
		return -1, fmt.Errorf("failed to open path %s: %w", path, err)
	}
	return fd, nil
}

// getAccessMaskForPolicy converts a SecurityPolicy's access level to a Landlock access mask.
// Read-only paths get read access, write paths get read+write access.
//
// @MX:ANCHOR: [AUTO] Convert policy access level to Landlock access mask
// @MX:REASON: Required for mapping high-level policy to kernel access rights, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func getAccessMaskForPolicy(isWrite bool) uint64 {
	if isWrite {
		// Write access: allow read, write, execute, directory operations
		return accessFSReadFile |
			accessFSReadDir |
			accessFSExecute |
			accessFSWriteFile |
			accessFSTruncate |
			accessFSRemoveFile |
			accessFSRemoveDir |
			accessFSMakeDir |
			accessFSMakeReg |
			accessFSMakeSym |
			accessFSMakeSock |
			accessFSMakeFifo |
			accessFSMakeBlock |
			accessFSMakeChar
	}

	// Read-only access: only allow read and execute
	return accessFSReadFile |
		accessFSReadDir |
		accessFSExecute
}

// addPathRule adds a path rule to the Landlock ruleset.
// It opens the path directory, creates a path-beneath rule, and adds it to the ruleset.
//
// @MX:ANCHOR: [AUTO] Add path rule to Landlock ruleset
// @MX:REASON: High-level function for adding path rules, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func addPathRule(rulesetFd int, path string, isWrite bool, abiVersion int) error {
	// Open the path directory
	fd, err := openPathFd(path)
	if err != nil {
		// If path doesn't exist, log but don't fail (optional paths)
		if os.IsNotExist(err) {
			return fmt.Errorf("path %s does not exist (optional)", path)
		}
		return fmt.Errorf("failed to open path %s: %w", path, err)
	}
	defer syscall.Close(fd)

	// Get the access mask for this policy
	accessMask := getAccessMaskForPolicy(isWrite)

	// Add ABI v2 rights if supported
	if abiVersion >= LandlockAbiVersion2 {
		accessMask |= accessFSRefer
	}

	// Create the path-beneath rule
	ruleAttr := &landlockPathBeneathAttr{
		AllowedAccess: accessMask,
		ParentFd:      int32(fd),
	}

	// Add the rule to the ruleset
	if err := landlockAddPathRule(rulesetFd, landlockRulePathBeneath, ruleAttr); err != nil {
		return fmt.Errorf("failed to add rule for path %s: %w", path, err)
	}

	return nil
}
