//go:build linux

// Package sandbox provides Linux Landlock LSM ABI constants and types.
// SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
package sandbox

import "syscall"

// Landlock syscall numbers (linux/amd64).
// These are the syscall numbers for Landlock operations on x86_64.
//
// @MX:NOTE: [AUTO] Landlock syscall numbers for Linux amd64
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
const (
	sysLandlockCreateRuleset uintptr = 444
	sysLandlockAddRule       uintptr = 445
	sysLandlockRestrictSelf  uintptr = 446
)

// Landlock access rights (ABI v1, kernel 5.13+).
// These bit flags define which filesystem operations are allowed by Landlock.
//
// @MX:NOTE: [AUTO] Landlock ABI v1 access rights
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
const (
	// LandlockAccessFSExecute executes a file.
	// @MX:NOTE: [AUTO] Execute file access right
	accessFSExecute uint64 = 1 << iota

	// LandlockAccessFSWriteFile writes a file.
	// @MX:NOTE: [AUTO] Write file access right
	accessFSWriteFile

	// LandlockAccessFSReadFile reads a file.
	// @MX:NOTE: [AUTO] Read file access right
	accessFSReadFile

	// LandlockAccessFSReadDir lists contents of a directory
	// or reads a directory symbolic link.
	// @MX:NOTE: [AUTO] Read directory access right
	accessFSReadDir

	// LandlockAccessFSRemoveDir removes an empty directory
	// or a symbolic link pointing to a directory.
	// @MX:NOTE: [AUTO] Remove directory access right
	accessFSRemoveDir

	// LandlockAccessFSRemoveFile removes a file or a symbolic link.
	// @MX:NOTE: [AUTO] Remove file access right
	accessFSRemoveFile

	// LandlockAccessFSMakeChar creates (or rename or link) a character device.
	// @MX:NOTE: [AUTO] Make character device access right
	accessFSMakeChar

	// LandlockAccessFSMakeDir creates (or rename or link) a directory.
	// @MX:NOTE: [AUTO] Make directory access right
	accessFSMakeDir

	// LandlockAccessFSMakeReg creates (or rename or link) a regular file.
	// @MX:NOTE: [AUTO] Make regular file access right
	accessFSMakeReg

	// LandlockAccessFSMakeSock creates (or rename or link) a UNIX domain socket.
	// @MX:NOTE: [AUTO] Make socket access right
	accessFSMakeSock

	// LandlockAccessFSMakeFifo creates (or rename or link) a named pipe.
	// @MX:NOTE: [AUTO] Make FIFO access right
	accessFSMakeFifo

	// LandlockAccessFSMakeBlock creates (or rename or link) a block device.
	// @MX:NOTE: [AUTO] Make block device access right
	accessFSMakeBlock

	// LandlockAccessFSMakeSym creates (or rename or link) a symbolic link.
	// @MX:NOTE: [AUTO] Make symbolic link access right
	accessFSMakeSym

	// LandlockAccessFSTruncate truncates a file with truncate(2).
	// @MX:NOTE: [AUTO] Truncate file access right
	accessFSTruncate
)

// Landlock ABI v2 additions (kernel 5.19+).
// These additional access rights are available in Landlock ABI v2.
//
// @MX:NOTE: [AUTO] Landlock ABI v2 access rights (kernel 5.19+)
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
const (
	// LandlockAccessFSRefer creates or links a file from a file descriptor
	// (e.g., using linkat with AT_EMPTY_PATH). This access right is available
	// in Landlock ABI v2 (kernel 5.19+).
	// @MX:NOTE: [AUTO] Refer file access right (ABI v2)
	accessFSRefer uint64 = 1 << 13
)

// LandlockRuleType defines the type of Landlock rule.
//
// @MX:NOTE: [AUTO] Landlock rule type constants
const (
	landlockRulePathBeneath uintptr = 1
)

// LandlockRulesetAttr represents the attributes for creating a Landlock ruleset.
// This structure is passed to the landlock_create_ruleset syscall.
//
// @MX:NOTE: [AUTO] Landlock ruleset attributes structure
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
type landlockRulesetAttr struct {
	// HandledAccessFS is a bitmask of access rights to be handled by this ruleset.
	// Only the access rights specified in this field will be restricted by the ruleset.
	// @MX:NOTE: [AUTO] Handled filesystem access rights bitmask
	HandledAccessFS uint64
}

// LandlockPathBeneathAttr represents a path-based rule for Landlock.
// This structure is used with landlock_add_rule to add a path rule to a ruleset.
//
// @MX:NOTE: [AUTO] Landlock path-beneath rule attributes
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
type landlockPathBeneathAttr struct {
	// AllowedAccess is a bitmask of access rights allowed for this path.
	// @MX:NOTE: [AUTO] Allowed access rights for this path
	AllowedAccess uint64

	// ParentFd is a file descriptor for the parent directory.
	// This must be opened with O_PATH | O_CLOEXEC.
	// @MX:NOTE: [AUTO] Parent directory file descriptor
	ParentFd int32
}

// Landlock ABI version constants.
//
// @MX:NOTE: [AUTO] Landlock ABI version constants
const (
	// LandlockAbiVersion1 is the initial ABI version (kernel 5.13+).
	// @MX:NOTE: [AUTO] Landlock ABI v1 (kernel 5.13+)
	LandlockAbiVersion1 = 1

	// LandlockAbiVersion2 is the second ABI version (kernel 5.19+).
	// @MX:NOTE: [AUTO] Landlock ABI v2 (kernel 5.19+)
	LandlockAbiVersion2 = 2
)

// LandlockFlags defines flags for Landlock syscalls.
//
// @MX:NOTE: [AUTO] Landlock syscall flags
const (
	// LandlockCreateRulesetFlagNoRestrictionSelf can be used with
	// landlock_create_ruleset to prevent the ruleset from being applied
	// to the calling thread. This is useful for testing.
	// @MX:NOTE: [AUTO] Flag to prevent automatic restriction of self
	LandlockCreateRulesetFlagNoRestrictionSelf = 1 << 0
)

// getLandlockAbiVersion detects the Landlock ABI version supported by the kernel.
// Returns 0 if Landlock is not available.
//
// @MX:ANCHOR: [AUTO] Landlock ABI version detection
// @MX:REASON: Required for runtime capability detection, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func getLandlockAbiVersion() int {
	// Try to detect ABI v2 first (kernel 5.19+)
	if isLandlockAbiSupported(LandlockAbiVersion2) {
		return LandlockAbiVersion2
	}

	// Try to detect ABI v1 (kernel 5.13+)
	if isLandlockAbiSupported(LandlockAbiVersion1) {
		return LandlockAbiVersion1
	}

	// Landlock not available
	return 0
}

// isLandlockAbiSupported checks if a specific Landlock ABI version is supported.
// It attempts to create a minimal ruleset with the ABI version's access rights.
//
// @MX:ANCHOR: [AUTO] Landlock ABI version support check
// @MX:REASON: Used by getLandlockAbiVersion for capability detection, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-SECURITY-SANDBOX-001 REQ-SANDBOX-003
func isLandlockAbiSupported(abiVersion int) bool {
	var handledAccessFS uint64

	// Select access rights based on ABI version
	switch abiVersion {
	case LandlockAbiVersion2:
		// ABI v2 includes all v1 rights plus accessFSRefer
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
	case LandlockAbiVersion1:
		// ABI v1 access rights
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
	default:
		return false
	}

	// Try to create a minimal ruleset to test ABI support
	attr := &landlockRulesetAttr{
		HandledAccessFS: handledAccessFS,
	}

	fd, err := landlockCreateRuleset(attr, unsafeSizeofLandlockRulesetAttr, 0)
	if err != nil {
		// Syscall failed, ABI version not supported
		return false
	}

	// Successfully created ruleset, ABI version is supported
	// Close the ruleset file descriptor
	_ = syscall.Close(fd)
	return true
}

// unsafeSizeofLandlockRulesetAttr returns the size of landlockRulesetAttr.
// Using a constant avoids importing unsafe.
//
// @MX:NOTE: [AUTO] Size of landlockRulesetAttr structure
const unsafeSizeofLandlockRulesetAttr uintptr = 8
