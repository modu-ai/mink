// Package onboarding — cli_detection_test.go tests the CLI tools PATH probe and
// version string parsing logic in cli_detection.go.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
// REQ: REQ-OB-023
package onboarding

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// -------------------------------------------------------------------------
// DetectCLITools tests
// -------------------------------------------------------------------------

func TestDetectCLITools_NoneInPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	tools, err := DetectCLITools(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tools == nil {
		t.Fatal("expected non-nil slice, got nil")
	}
	if len(tools) != 0 {
		t.Errorf("expected empty slice, got %d tools: %+v", len(tools), tools)
	}
}

func TestDetectCLITools_ClaudeOnly_VersionParsed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on Windows")
	}
	stubDir := t.TempDir()
	writeCLIStub(t, stubDir, "claude", "echo 'claude version 1.2.3'")
	t.Setenv("PATH", stubDir)

	tools, err := DetectCLITools(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "claude" {
		t.Errorf("expected Name=claude, got: %s", tools[0].Name)
	}
	if tools[0].Version != "1.2.3" {
		t.Errorf("expected Version=1.2.3, got: %s", tools[0].Version)
	}
	if tools[0].Path == "" {
		t.Error("expected non-empty Path")
	}
}

func TestDetectCLITools_AllThreeTools(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on Windows")
	}
	stubDir := t.TempDir()
	writeCLIStub(t, stubDir, "claude", "echo '1.0.0'")
	writeCLIStub(t, stubDir, "gemini", "echo 'gemini 2.0.0-beta'")
	writeCLIStub(t, stubDir, "codex", "echo 'codex/0.5.0'")
	t.Setenv("PATH", stubDir)

	tools, err := DetectCLITools(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %+v", len(tools), tools)
	}

	versionsByName := make(map[string]string)
	for _, tool := range tools {
		versionsByName[tool.Name] = tool.Version
	}
	if versionsByName["claude"] != "1.0.0" {
		t.Errorf("claude version mismatch: %s", versionsByName["claude"])
	}
	if versionsByName["gemini"] != "2.0.0-beta" {
		t.Errorf("gemini version mismatch: %s", versionsByName["gemini"])
	}
	if versionsByName["codex"] != "0.5.0" {
		t.Errorf("codex version mismatch: %s", versionsByName["codex"])
	}
}

func TestDetectCLITools_VersionParseFailure_StillIncluded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on Windows")
	}
	stubDir := t.TempDir()
	writeCLIStub(t, stubDir, "claude", "echo 'hello world'")
	t.Setenv("PATH", stubDir)

	tools, err := DetectCLITools(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d: %+v", len(tools), tools)
	}
	if tools[0].Name != "claude" {
		t.Errorf("expected Name=claude, got: %s", tools[0].Name)
	}
	// Version parse should fail gracefully, giving empty string.
	if tools[0].Version != "" {
		t.Errorf("expected empty Version on parse failure, got: %s", tools[0].Version)
	}
	if tools[0].Path == "" {
		t.Error("expected non-empty Path even when version parse fails")
	}
}

func TestDetectCLITools_ExecLookPathOverride(t *testing.T) {
	orig := execLookPath
	execLookPath = func(name string) (string, error) {
		if name == "claude" {
			return "/fake/bin/claude", nil
		}
		return "", fmt.Errorf("not found: %s", name)
	}
	t.Cleanup(func() { execLookPath = orig })

	// Also stub the version exec so no real binary is called.
	origExec := execCommandContext
	execCommandContext = fakeExecCommand("claude version 9.8.7", 0)
	t.Cleanup(func() { execCommandContext = origExec })

	tools, err := DetectCLITools(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Path != "/fake/bin/claude" {
		t.Errorf("expected /fake/bin/claude, got: %s", tools[0].Path)
	}
	if tools[0].Version != "9.8.7" {
		t.Errorf("expected version 9.8.7, got: %s", tools[0].Version)
	}
}

func TestDetectCLITools_TimeoutHandled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on Windows")
	}
	// Override lookpath to find a fake claude.
	orig := execLookPath
	execLookPath = func(name string) (string, error) {
		if name == "claude" {
			return "/fake/bin/claude", nil
		}
		return "", fmt.Errorf("not found: %s", name)
	}
	t.Cleanup(func() { execLookPath = orig })

	// Stub exec to run "sleep 60" so the context timeout fires.
	origExec := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, "sleep", "60")
	}
	t.Cleanup(func() { execCommandContext = origExec })

	// Use a very short timeout for the version detection.
	origTimeout := versionExecTimeout
	versionExecTimeout = 50 // 50 ms — enough to trigger timeout
	t.Cleanup(func() { versionExecTimeout = origTimeout })

	tools, err := DetectCLITools(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// claude should be present but with empty version.
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool (with empty version), got %d", len(tools))
	}
	if tools[0].Version != "" {
		t.Errorf("expected empty version on timeout, got: %s", tools[0].Version)
	}
}

// -------------------------------------------------------------------------
// ParseToolVersion tests
// -------------------------------------------------------------------------

func TestParseToolVersion_Basic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"claude version 1.2.3", "1.2.3"},
		{"gemini 2.0.0-beta.1 (commit abc)", "2.0.0-beta.1"},
		{"codex/0.5.0", "0.5.0"},
		{"no version here", ""},
		{"", ""},
		{"v1.2", ""}, // incomplete SemVer — regex requires X.Y.Z
	}
	for _, tc := range tests {
		got := ParseToolVersion(tc.input)
		if got != tc.want {
			t.Errorf("ParseToolVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseToolVersion_FirstMatchWins(t *testing.T) {
	got := ParseToolVersion("old 0.1.0 new 1.0.0")
	if got != "0.1.0" {
		t.Errorf("expected first match 0.1.0, got: %s", got)
	}
}

// -------------------------------------------------------------------------
// Helper: write a shell script stub for CLI tools.
// -------------------------------------------------------------------------

func writeCLIStub(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on Windows")
	}
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writeCLIStub: %v", err)
	}
	return path
}
