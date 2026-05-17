// Package auth — static analysis tests for AGPL §2 compliance and ADR-001
// telemetry plaintext prevention.
//
// These tests use os/exec + go list for the import-graph check and regexp
// scanning for the telemetry plaintext check.  They are plain Go tests with
// no build tags, so they run in every `go test ./internal/auth/...` invocation.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UB-3, UB-5, UN-5, T-014, AC-CR-003, AC-CR-010)
package auth_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestAGPLImportGraphCompliance verifies that no file under internal/auth/...
// imports paths that contain model training packages.
//
// AC-CR-003: credential code must not import internal/model/training/**
// or any path matching learning/training or model/train.
func TestAGPLImportGraphCompliance(t *testing.T) {
	// Skip gracefully if the `go` binary is not available (unlikely in CI).
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not on PATH; skipping import-graph test")
	}

	// Use `go list` to enumerate all transitive imports of internal/auth/...
	cmd := exec.Command(goPath, "list", "-deps", "-f", `{{join .Imports "\n"}}`, "./internal/auth/...")
	cmd.Dir = findModuleRoot(t)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Logf("go list stderr: %s", stderr.String())
		t.Fatalf("go list failed: %v", err)
	}

	// Prohibited import path patterns (AGPL §2 — credential code ⊥ model training).
	prohibited := []string{
		"internal/model/training",
		"learning/training",
		"model/train",
	}

	var offenders []string
	for line := range strings.SplitSeq(stdout.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, pattern := range prohibited {
			if strings.Contains(line, pattern) {
				offenders = append(offenders, line)
				break
			}
		}
	}

	if len(offenders) > 0 {
		t.Errorf("AGPL §2 violation: internal/auth imports model training packages:\n  %s",
			strings.Join(offenders, "\n  "))
	}
}

// TestTelemetryPlaintextLeakAbsent scans all non-test Go source files under
// internal/auth/ for patterns that could transmit credential plaintext via
// telemetry or metrics calls.
//
// AC-CR-010: refresh_token / access_token / signing secrets must not appear
// in telemetry.Send / telemetry.Emit / metrics.Track / metrics.Record calls.
func TestTelemetryPlaintextLeakAbsent(t *testing.T) {
	// Pattern: telemetry/metrics call followed on the same line by a credential
	// field name.  This is a defensive grep, not an AST analysis.
	pattern := regexp.MustCompile(
		`(?i)(telemetry\.(Send|Emit)|metrics\.(Track|Record)).*` +
			`(AccessToken|RefreshToken|Token|SigningSecret|Value\b)`,
	)

	root := findModuleRoot(t)
	authDir := filepath.Join(root, "internal", "auth")

	var violations []string
	err := filepath.WalkDir(authDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		// Skip test files and generated files.
		if strings.HasSuffix(name, "_test.go") ||
			strings.HasSuffix(name, "_generated.go") {
			return nil
		}
		if !strings.HasSuffix(name, ".go") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}

		for i, line := range bytes.Split(data, []byte("\n")) {
			if pattern.Match(line) {
				violations = append(violations, fmt.Sprintf("%s:%d: %s", path, i+1, strings.TrimSpace(string(line))))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk internal/auth: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("ADR-001 violation: potential telemetry plaintext leak in %d location(s):\n  %s",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// findModuleRoot walks up from the test working directory to find the Go
// module root (identified by the presence of go.mod).
func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod by walking up from test working directory")
		}
		dir = parent
	}
}
