// Package onboarding — paths.go provides canonical path resolution for the MINK
// file storage layout described in SPEC-MINK-ONBOARDING-001 §6.0.
//
// Two root directories are distinguished:
//
//   - Global config dir (~/.mink/): user-level, shared across projects.
//     Determined by $MINK_HOME override → os.UserHomeDir() → $HOME fallback.
//
//   - Project workspace dir (./.mink/): per-project, located via upward traversal
//     from the current working directory. Determined by $MINK_PROJECT_DIR override
//     → upward traversal from os.Getwd() → cwd/.mink fallback.
//
// All functions in this file are pure path resolvers — no file I/O is performed.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.0, §6.1
// REQ: REQ-OB-009, REQ-OB-011, REQ-OB-017, REQ-OB-027
package onboarding

import (
	"errors"
	"os"
	"path/filepath"
)

// File and directory name constants for the MINK storage layout.
const (
	// GlobalDirName is the directory name under $HOME used for global config.
	GlobalDirName = ".mink"

	// ProjectDirName is the directory name searched via upward traversal for
	// project-scoped storage.
	ProjectDirName = ".mink"

	// GlobalConfigFile is the config file name inside the global dir.
	GlobalConfigFile = "config.yaml"

	// ProjectConfigFile is the config file name inside the project dir.
	ProjectConfigFile = "config.yaml"

	// DraftFile is the filename for the paused onboarding wizard state.
	DraftFile = "onboarding-draft.yaml"

	// SecurityEventsFile is the append-only log for REQ-OB-017 security events.
	SecurityEventsFile = "security-events.log"

	// OnboardingCompletedFile is a timestamp marker written after successful
	// onboarding completion (written by Phase 1E).
	OnboardingCompletedFile = "onboarding-completed"
)

// Sentinel errors returned by path resolver functions.
var (
	// ErrHomeNotFound is returned by GlobalConfigDir when neither $MINK_HOME
	// nor $HOME is set and os.UserHomeDir() fails.
	ErrHomeNotFound = errors.New("$HOME and $MINK_HOME are both unset")

	// ErrCWDNotFound is returned by ProjectConfigDir when os.Getwd() fails and
	// $MINK_PROJECT_DIR is not set.
	ErrCWDNotFound = errors.New("unable to determine current working directory")
)

// GlobalConfigDir returns the absolute path to the global MINK config directory
// (~/.mink/ by default).
//
// Resolution order:
//  1. $MINK_HOME env var (allows tests and containerised usage to redirect)
//  2. os.UserHomeDir()
//  3. $HOME env var (last fallback for restricted environments)
//
// Returns ErrHomeNotFound if none of the above is available.
func GlobalConfigDir() (string, error) {
	if v := os.Getenv("MINK_HOME"); v != "" {
		return filepath.Clean(v), nil
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, GlobalDirName), nil
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, GlobalDirName), nil
	}
	return "", ErrHomeNotFound
}

// GlobalConfigPath returns the absolute path to ~/.mink/config.yaml.
func GlobalConfigPath() (string, error) {
	dir, err := GlobalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, GlobalConfigFile), nil
}

// ProjectConfigDir returns the absolute path to the project's .mink/ directory.
//
// Search order:
//  1. $MINK_PROJECT_DIR env var (escape hatch for tests and explicit project
//     pinning — the value is used as-is, no GlobalDirName appended).
//  2. Upward traversal from os.Getwd(): walks parent directories looking for
//     a directory named ".mink". Returns the first match found.
//  3. os.Getwd() + "/.mink" fallback — returned even if the directory does not
//     yet exist, so callers can use the path to create it.
//
// This function does NOT create the directory.
// Returns ErrCWDNotFound only when os.Getwd() itself fails and $MINK_PROJECT_DIR
// is unset.
//
// @MX:NOTE: [AUTO] Upward-traversal contract: matches the README.md "Storage layout"
// intent (./.mink/ is per-project, discovered by walking up from cwd).
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6.0
func ProjectConfigDir() (string, error) {
	if v := os.Getenv("MINK_PROJECT_DIR"); v != "" {
		return filepath.Clean(v), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", ErrCWDNotFound
	}
	// Upward traversal: walk from cwd toward filesystem root.
	dir := cwd
	for {
		candidate := filepath.Join(dir, ProjectDirName)
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding .mink/.
			break
		}
		dir = parent
	}
	// Fallback: return expected location under cwd even if it does not exist yet.
	return filepath.Join(cwd, ProjectDirName), nil
}

// ProjectConfigPath returns the absolute path to <project>/.mink/config.yaml.
func ProjectConfigPath() (string, error) {
	dir, err := ProjectConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ProjectConfigFile), nil
}

// DraftPath returns the absolute path to <project>/.mink/onboarding-draft.yaml.
func DraftPath() (string, error) {
	dir, err := ProjectConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DraftFile), nil
}

// SecurityEventsPath returns the absolute path to <project>/.mink/security-events.log.
func SecurityEventsPath() (string, error) {
	dir, err := ProjectConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, SecurityEventsFile), nil
}

// OnboardingCompletedPath returns the absolute path to <project>/.mink/onboarding-completed.
func OnboardingCompletedPath() (string, error) {
	dir, err := ProjectConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, OnboardingCompletedFile), nil
}
