// Package onboarding — cli_detection.go probes the host PATH for known CLI
// delegation targets (claude, gemini, codex) and extracts their version strings.
// Detection results are stored in CLIToolsDetection (defined in types.go).
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
// REQ: REQ-OB-023
package onboarding

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// Sentinel error returned when version parsing fails.
var ErrToolVersionParse = errors.New("cli_detection: version parse failed")

// toolNames lists the CLI delegation targets probed by DetectCLITools.
var toolNames = []string{"claude", "gemini", "codex"}

// semVerRe matches the first SemVer-like token of the form X.Y.Z or X.Y.Z-suffix.
var semVerRe = regexp.MustCompile(`\d+\.\d+\.\d+(-[A-Za-z0-9.\-]+)?`)

// execLookPath is exec.LookPath, indirected for test injection.
var execLookPath = exec.LookPath

// versionExecTimeout is the timeout in milliseconds used when running `<tool> --version`.
// Exposed as a package-level variable so tests can shorten it.
var versionExecTimeout = 3000

// DetectCLITools probes the system PATH for claude / gemini / codex CLI binaries
// and returns those that are present. For each found tool, attempts to extract
// a SemVer-like version string from `<tool> --version` stdout.
// Tools not in PATH are omitted from the result (no entry with empty Path).
// Always returns a non-nil slice (empty when no tool is found).
// Version parse failures still include the tool with Version="".
// Returns nil error on the normal "no tools found" path; only returns error
// for truly exceptional failures (none currently expected, reserved for future
// extension — for v1, the error return is always nil).
//
// @MX:ANCHOR: [AUTO] Primary public entry point for CLI tool detection — called by
// flow.go step 3 assignStepData and potentially Web UI handler.
// @MX:REASON: Changing the signature or semantics breaks all callers simultaneously.
func DetectCLITools(ctx context.Context) ([]CLITool, error) {
	var detected []CLITool

	for _, name := range toolNames {
		path, err := execLookPath(name)
		if err != nil {
			// Binary not in PATH — omit from results entirely.
			continue
		}

		version := probeToolVersion(ctx, name, path)
		detected = append(detected, CLITool{
			Name:    name,
			Version: version,
			Path:    path,
		})
	}

	if detected == nil {
		detected = []CLITool{}
	}
	return detected, nil
}

// ParseToolVersion extracts the first SemVer-like token from the given output.
// Pattern: \d+\.\d+\.\d+(-[A-Za-z0-9\.-]+)?
// Returns the matched token, or "" if no match.
func ParseToolVersion(output string) string {
	match := semVerRe.FindString(output)
	return match
}

// probeToolVersion runs `<name> --version` with a short timeout and parses stdout.
// Returns an empty string on timeout, non-zero exit, or unparseable output.
// The tool is still included in the result even when version is empty.
func probeToolVersion(ctx context.Context, name, _ string) string {
	timeout := time.Duration(versionExecTimeout) * time.Millisecond
	vctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := execCommandContext(vctx, name, "--version")
	out, err := cmd.Output()
	if err != nil {
		// Timeout or non-zero exit — return empty version, tool still included.
		return ""
	}

	return ParseToolVersion(strings.TrimSpace(string(out)))
}
