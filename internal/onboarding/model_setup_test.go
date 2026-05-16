// Package onboarding — model_setup_test.go tests runtime detection helpers.
// Covers DetectOllama, DetectMINKModel, DetectRAM, RecommendModel, and PullModel.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
// REQ: REQ-OB-021, REQ-CP-010, REQ-CP-011
package onboarding

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// GiB is 1024^3 bytes, used in RecommendModel boundary tests.
const GiB = int64(1024 * 1024 * 1024)

// -------------------------------------------------------------------------
// TestHelperProcess: well-known TestMain trick for fake exec binaries.
// The test binary re-invokes itself with GO_WANT_HELPER_PROCESS=1 to
// simulate a subprocess. Tests should NOT call this directly.
// -------------------------------------------------------------------------

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	output := os.Getenv("FAKE_STDOUT")
	code, _ := strconv.Atoi(os.Getenv("FAKE_EXIT"))
	if output != "" {
		fmt.Print(output)
	}
	os.Exit(code)
}

// fakeExecCommand returns an execCommandContext stub that prints stdout and
// exits with exitCode when invoked as a subprocess via TestHelperProcess.
func fakeExecCommand(stdout string, exitCode int) func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{
			"GO_WANT_HELPER_PROCESS=1",
			"FAKE_STDOUT=" + stdout,
			"FAKE_EXIT=" + strconv.Itoa(exitCode),
		}
		return cmd
	}
}

// -------------------------------------------------------------------------
// DetectOllama tests
// -------------------------------------------------------------------------

func TestDetectOllama_NotInPATH(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	status, err := DetectOllama(context.Background())

	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if status.Installed {
		t.Error("expected Installed=false when ollama not in PATH")
	}
	if status.BinaryPath != "" {
		t.Errorf("expected empty BinaryPath, got: %s", status.BinaryPath)
	}
	if status.DaemonAlive {
		t.Error("expected DaemonAlive=false when ollama not installed")
	}
}

func TestDetectOllama_PresentInPATH_DaemonAlive(t *testing.T) {
	// Create a stub ollama binary that exits 0.
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on Windows")
	}
	stubDir := t.TempDir()
	writeStubScript(t, stubDir, "ollama", "exit 0")
	t.Setenv("PATH", stubDir)

	// Mock httpGet to return 200 with empty model list.
	orig := httpGet
	httpGet = func(ctx context.Context, url string) (*http.Response, error) {
		body := `{"models":[]}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
	t.Cleanup(func() { httpGet = orig })

	status, err := DetectOllama(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Installed {
		t.Error("expected Installed=true")
	}
	if !status.DaemonAlive {
		t.Error("expected DaemonAlive=true")
	}
	if status.BinaryPath == "" {
		t.Error("expected non-empty BinaryPath")
	}
}

func TestDetectOllama_PresentInPATH_DaemonDown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on Windows")
	}
	stubDir := t.TempDir()
	writeStubScript(t, stubDir, "ollama", "exit 0")
	t.Setenv("PATH", stubDir)

	// Mock httpGet to return a network error.
	orig := httpGet
	httpGet = func(ctx context.Context, url string) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}
	t.Cleanup(func() { httpGet = orig })

	status, err := DetectOllama(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Installed {
		t.Error("expected Installed=true")
	}
	if status.DaemonAlive {
		t.Error("expected DaemonAlive=false when HTTP fails")
	}
}

// -------------------------------------------------------------------------
// DetectMINKModel tests
// -------------------------------------------------------------------------

func TestDetectMINKModel_HTTPSuccess_ReturnsFirstMINKModel(t *testing.T) {
	orig := httpGet
	httpGet = func(ctx context.Context, url string) (*http.Response, error) {
		body := `{"models":[{"name":"ai-mink/gemma4-e4b-rl-v1:q5_k_m","size":4500000000},{"name":"llama2","size":3000000000}]}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
	t.Cleanup(func() { httpGet = orig })

	status := OllamaStatus{Installed: true, DaemonAlive: true}
	model, err := DetectMINKModel(context.Background(), status)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.Name != "ai-mink/gemma4-e4b-rl-v1:q5_k_m" {
		t.Errorf("expected ai-mink model, got: %s", model.Name)
	}
	if model.SizeBytes != 4500000000 {
		t.Errorf("expected SizeBytes=4500000000, got: %d", model.SizeBytes)
	}
}

func TestDetectMINKModel_HTTPSuccess_NoMINKModel(t *testing.T) {
	orig := httpGet
	httpGet = func(ctx context.Context, url string) (*http.Response, error) {
		body := `{"models":[{"name":"llama2","size":3000000000},{"name":"mistral","size":4000000000}]}`
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}
	t.Cleanup(func() { httpGet = orig })

	status := OllamaStatus{Installed: true, DaemonAlive: true}
	model, err := DetectMINKModel(context.Background(), status)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.Name != "" {
		t.Errorf("expected empty Name, got: %s", model.Name)
	}
	if model.SizeBytes != 0 {
		t.Errorf("expected SizeBytes=0, got: %d", model.SizeBytes)
	}
}

func TestDetectMINKModel_HTTPFails_FallsBackToExec(t *testing.T) {
	// HTTP fails — daemon not running.
	origHTTP := httpGet
	httpGet = func(ctx context.Context, url string) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}
	t.Cleanup(func() { httpGet = origHTTP })

	// Exec fallback returns valid ollama list output.
	origExec := execCommandContext
	ollamaListOutput := "NAME\tID\tSIZE\tMODIFIED\nai-mink/gemma4-e4b-rl-v1:q5_k_m\tabc123\t4.2 GB\t3 days ago\n"
	execCommandContext = fakeExecCommand(ollamaListOutput, 0)
	t.Cleanup(func() { execCommandContext = origExec })

	status := OllamaStatus{Installed: true, DaemonAlive: false}
	model, err := DetectMINKModel(context.Background(), status)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.Name != "ai-mink/gemma4-e4b-rl-v1:q5_k_m" {
		t.Errorf("expected ai-mink model from exec fallback, got: %s", model.Name)
	}
}

func TestDetectMINKModel_BothMethodsFail(t *testing.T) {
	origHTTP := httpGet
	httpGet = func(ctx context.Context, url string) (*http.Response, error) {
		return nil, errors.New("connection refused")
	}
	t.Cleanup(func() { httpGet = origHTTP })

	origExec := execCommandContext
	execCommandContext = fakeExecCommand("", 1)
	t.Cleanup(func() { execCommandContext = origExec })

	status := OllamaStatus{Installed: true, DaemonAlive: false}
	_, err := DetectMINKModel(context.Background(), status)

	if err == nil {
		t.Fatal("expected error when both methods fail, got nil")
	}
	if !errors.Is(err, ErrModelDetection) {
		t.Errorf("expected ErrModelDetection, got: %v", err)
	}
}

func TestDetectMINKModel_OllamaNotInstalled_ReturnsEmpty(t *testing.T) {
	// When ollama is not installed, no HTTP or exec probe should be attempted.
	origHTTP := httpGet
	httpGet = func(ctx context.Context, url string) (*http.Response, error) {
		t.Error("httpGet should not be called when ollama is not installed")
		return nil, errors.New("should not be called")
	}
	t.Cleanup(func() { httpGet = origHTTP })

	origExec := execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		t.Error("execCommandContext should not be called when ollama is not installed")
		return exec.CommandContext(ctx, name, args...)
	}
	t.Cleanup(func() { execCommandContext = origExec })

	status := OllamaStatus{Installed: false}
	model, err := DetectMINKModel(context.Background(), status)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.Name != "" || model.SizeBytes != 0 {
		t.Errorf("expected empty DetectedModel, got: %+v", model)
	}
}

// -------------------------------------------------------------------------
// DetectRAM tests
// -------------------------------------------------------------------------

func TestDetectRAM_OverrideHook(t *testing.T) {
	orig := detectRAMOverride
	detectRAMOverride = func(_ context.Context) (int64, error) {
		return 16 * GiB, nil
	}
	t.Cleanup(func() { detectRAMOverride = orig })

	ram, err := DetectRAM(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ram != 16*GiB {
		t.Errorf("expected 16 GiB, got: %d", ram)
	}
}

func TestDetectRAM_UnsupportedOS(t *testing.T) {
	// Clear the override so real platform dispatch is used.
	orig := detectRAMOverride
	detectRAMOverride = nil
	t.Cleanup(func() { detectRAMOverride = orig })

	goos := runtime.GOOS
	switch goos {
	case "darwin", "linux":
		// On these platforms, we expect non-zero RAM and no error.
		ram, err := DetectRAM(context.Background())
		if err != nil {
			t.Fatalf("expected nil error on %s, got: %v", goos, err)
		}
		if ram <= 0 {
			t.Errorf("expected positive RAM on %s, got: %d", goos, ram)
		}
	case "windows":
		t.Skip("Windows WMIC detection may not be available in CI")
	default:
		// Unknown GOOS should return ErrUnsupportedOS.
		_, err := DetectRAM(context.Background())
		if err == nil {
			t.Fatal("expected error for unsupported GOOS")
		}
		if !errors.Is(err, ErrUnsupportedOS) {
			t.Errorf("expected ErrUnsupportedOS, got: %v", err)
		}
	}
}

// -------------------------------------------------------------------------
// RecommendModel tests
// -------------------------------------------------------------------------

func TestRecommendModel_LessThan8GB(t *testing.T) {
	got := RecommendModel(1 * GiB)
	if got != "ai-mink/gemma4-e2b-rl-v1" {
		t.Errorf("expected e2b model, got: %s", got)
	}
}

func TestRecommendModel_8To16GB(t *testing.T) {
	got := RecommendModel(12 * GiB)
	if got != "ai-mink/gemma4-e4b-rl-v1:q4_k_m" {
		t.Errorf("expected q4_k_m model, got: %s", got)
	}
}

func TestRecommendModel_16To32GB(t *testing.T) {
	got := RecommendModel(20 * GiB)
	if got != "ai-mink/gemma4-e4b-rl-v1:q5_k_m" {
		t.Errorf("expected q5_k_m model, got: %s", got)
	}
}

func TestRecommendModel_32GBOrMore(t *testing.T) {
	got := RecommendModel(64 * GiB)
	if got != "ai-mink/gemma4-e4b-rl-v1:q8_0" {
		t.Errorf("expected q8_0 model, got: %s", got)
	}
}

func TestRecommendModel_Boundaries(t *testing.T) {
	tests := []struct {
		ram  int64
		want string
	}{
		{8 * GiB, "ai-mink/gemma4-e4b-rl-v1:q4_k_m"},
		{16 * GiB, "ai-mink/gemma4-e4b-rl-v1:q5_k_m"},
		{32 * GiB, "ai-mink/gemma4-e4b-rl-v1:q8_0"},
		{8*GiB - 1, "ai-mink/gemma4-e2b-rl-v1"},
		{16*GiB - 1, "ai-mink/gemma4-e4b-rl-v1:q4_k_m"},
		{32*GiB - 1, "ai-mink/gemma4-e4b-rl-v1:q5_k_m"},
	}
	for _, tc := range tests {
		got := RecommendModel(tc.ram)
		if got != tc.want {
			t.Errorf("RecommendModel(%d) = %q, want %q", tc.ram, got, tc.want)
		}
	}
}

// -------------------------------------------------------------------------
// PullModel tests
// -------------------------------------------------------------------------

func TestPullModel_HappyPath(t *testing.T) {
	origExec := execCommandContext
	output := "pulling manifest\npulling abc1234... 50% ▕███░░▏ 1.5 GB/3.0 GB\nsuccess\n"
	execCommandContext = fakeExecCommand(output, 0)
	t.Cleanup(func() { execCommandContext = origExec })

	progress := make(chan ProgressUpdate, 20)
	err := PullModel(context.Background(), "ai-mink/test-model:v1", progress)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var updates []ProgressUpdate
	for u := range progress {
		updates = append(updates, u)
	}

	// Expect at least 3 ProgressUpdates: manifest, downloading, success.
	if len(updates) < 3 {
		t.Errorf("expected at least 3 progress updates, got %d: %+v", len(updates), updates)
	}

	// Verify success is the last phase.
	last := updates[len(updates)-1]
	if last.Phase != "success" {
		t.Errorf("expected last phase=success, got: %s", last.Phase)
	}
}

func TestPullModel_NonZeroExit_ReturnsErrPullFailed(t *testing.T) {
	origExec := execCommandContext
	execCommandContext = fakeExecCommand("error: model not found\n", 1)
	t.Cleanup(func() { execCommandContext = origExec })

	progress := make(chan ProgressUpdate, 10)
	err := PullModel(context.Background(), "ai-mink/nonexistent:v1", progress)

	// Drain the channel.
	for range progress {
	}

	if err == nil {
		t.Fatal("expected error on non-zero exit, got nil")
	}
	if !errors.Is(err, ErrPullFailed) {
		t.Errorf("expected ErrPullFailed, got: %v", err)
	}
}

func TestPullModel_ContextCancellation(t *testing.T) {
	origExec := execCommandContext
	// A fake command that blocks (sleeps) — we cancel the context before it finishes.
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		// Use "sleep" to simulate a long-running pull.
		return exec.CommandContext(ctx, "sleep", "60")
	}
	t.Cleanup(func() { execCommandContext = origExec })

	ctx, cancel := context.WithCancel(context.Background())
	progress := make(chan ProgressUpdate, 10)

	// Cancel immediately.
	cancel()

	err := PullModel(ctx, "ai-mink/test:v1", progress)

	// Drain any buffered updates.
	for range progress {
	}

	if err == nil {
		t.Fatal("expected error on context cancellation, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// -------------------------------------------------------------------------
// Helper: write a shell script stub binary.
// -------------------------------------------------------------------------

func writeStubScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script stubs not supported on Windows")
	}
	path := fmt.Sprintf("%s/%s", dir, name)
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writeStubScript: %v", err)
	}
	return path
}
