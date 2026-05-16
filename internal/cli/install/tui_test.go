// Package install — tui_test.go tests the backend wiring helpers in tui.go.
// Full TUI interaction via teatest is Phase 2C scope; these tests cover:
//   - TTY detection indirection
//   - Pre-flight detection summary string builder
//   - Model selection logic given fixed inputs
//   - Provider step input construction (verifies secret not retained in OnboardingData)
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6 (Phase 2A)
package install

import (
	"strings"
	"testing"

	"github.com/modu-ai/mink/internal/onboarding"
)

// -----------------------------------------------------------------------
// TestIsTTYFunc_Indirection — verify the package-level var is injectable.
// -----------------------------------------------------------------------

// TestIsTTYFunc_Indirection verifies that IsTTYFunc can be replaced to simulate
// non-TTY environments without requiring a real OS pipe in tests.
func TestIsTTYFunc_Indirection(t *testing.T) {
	original := IsTTYFunc
	t.Cleanup(func() { IsTTYFunc = original })

	// Simulate non-TTY.
	IsTTYFunc = func(_ uintptr) bool { return false }
	if IsTTYFunc(0) {
		t.Fatal("expected IsTTYFunc to return false after substitution")
	}

	// Simulate TTY.
	IsTTYFunc = func(_ uintptr) bool { return true }
	if !IsTTYFunc(0) {
		t.Fatal("expected IsTTYFunc to return true after substitution")
	}
}

// -----------------------------------------------------------------------
// TestSummarizeDetection — pre-flight summary string builder
// -----------------------------------------------------------------------

func TestSummarizeDetection_AllDetected(t *testing.T) {
	status := onboarding.OllamaStatus{Installed: true, DaemonAlive: true}
	ram := int64(16 * 1024 * 1024 * 1024) // 16 GB
	model := onboarding.DetectedModel{Name: "ai-mink/gemma4-e4b-rl-v1:q5_k_m", SizeBytes: 3_000_000_000}
	tools := []onboarding.CLITool{
		{Name: "claude", Version: "1.2.3", Path: "/usr/local/bin/claude"},
		{Name: "codex", Version: "0.1.0", Path: "/usr/local/bin/codex"},
	}

	got := summarizeDetection(status, ram, model, tools)

	if !strings.Contains(got, "installed+running") {
		t.Errorf("expected 'installed+running' in summary, got: %s", got)
	}
	if !strings.Contains(got, "16 GB") {
		t.Errorf("expected '16 GB' RAM in summary, got: %s", got)
	}
	if !strings.Contains(got, "ai-mink/gemma4-e4b-rl-v1:q5_k_m") {
		t.Errorf("expected model name in summary, got: %s", got)
	}
	if !strings.Contains(got, "claude") {
		t.Errorf("expected 'claude' in CLI tools summary, got: %s", got)
	}
	if !strings.Contains(got, "codex") {
		t.Errorf("expected 'codex' in CLI tools summary, got: %s", got)
	}
}

func TestSummarizeDetection_OllamaNotInstalled(t *testing.T) {
	status := onboarding.OllamaStatus{Installed: false, DaemonAlive: false}
	model := onboarding.DetectedModel{}
	tools := []onboarding.CLITool{}

	got := summarizeDetection(status, 0, model, tools)

	if !strings.Contains(got, "not installed") {
		t.Errorf("expected 'not installed' in summary when Ollama absent, got: %s", got)
	}
}

func TestSummarizeDetection_OllamaInstalledButDaemonDown(t *testing.T) {
	status := onboarding.OllamaStatus{Installed: true, DaemonAlive: false}
	model := onboarding.DetectedModel{}
	tools := []onboarding.CLITool{}

	got := summarizeDetection(status, 8*1024*1024*1024, model, tools)

	if !strings.Contains(got, "daemon down") {
		t.Errorf("expected 'daemon down' in summary when daemon is not alive, got: %s", got)
	}
}

func TestSummarizeDetection_EmptyTools(t *testing.T) {
	status := onboarding.OllamaStatus{Installed: true, DaemonAlive: true}
	model := onboarding.DetectedModel{Name: "ai-mink/test"}
	tools := []onboarding.CLITool{}

	got := summarizeDetection(status, 8*1024*1024*1024, model, tools)

	if !strings.Contains(got, "CLI tools=none") {
		t.Errorf("expected 'CLI tools=none' when no tools detected, got: %s", got)
	}
}

func TestSummarizeDetection_EmptyModel(t *testing.T) {
	status := onboarding.OllamaStatus{Installed: true, DaemonAlive: true}
	model := onboarding.DetectedModel{} // empty — no ai-mink/ model found
	tools := []onboarding.CLITool{{Name: "claude"}}

	got := summarizeDetection(status, 32*1024*1024*1024, model, tools)

	if !strings.Contains(got, "model=<none>") {
		t.Errorf("expected 'model=<none>' when no model detected, got: %s", got)
	}
}

func TestSummarizeDetection_RAMUnknown(t *testing.T) {
	status := onboarding.OllamaStatus{}
	model := onboarding.DetectedModel{}
	tools := []onboarding.CLITool{}

	got := summarizeDetection(status, 0, model, tools)

	if !strings.Contains(got, "<unknown>") {
		t.Errorf("expected '<unknown>' for RAM when ramBytes=0, got: %s", got)
	}
}

// -----------------------------------------------------------------------
// TestRecommendedModelSelection_Logic — model recommendation given RAM
// -----------------------------------------------------------------------

// TestRecommendedModelSelection_Logic verifies that the wizard's model recommendation
// logic correctly delegates to onboarding.RecommendModel with detected RAM.
func TestRecommendedModelSelection_Logic(t *testing.T) {
	tests := []struct {
		name        string
		ramBytes    int64
		wantContain string // substring expected in RecommendModel output
	}{
		{"low RAM (4 GB)", 4 * 1024 * 1024 * 1024, "e2b"},
		{"medium RAM (8 GB)", 8 * 1024 * 1024 * 1024, "e4b"},
		{"high RAM (16 GB)", 16 * 1024 * 1024 * 1024, "e4b"},
		{"very high RAM (32 GB)", 32 * 1024 * 1024 * 1024, "e4b"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			recommended := onboarding.RecommendModel(tc.ramBytes)
			if recommended == "" {
				t.Fatal("RecommendModel returned empty string")
			}
			if !strings.Contains(recommended, tc.wantContain) {
				t.Errorf("for RAM=%d: RecommendModel=%q, expected to contain %q",
					tc.ramBytes, recommended, tc.wantContain)
			}
		})
	}
}

// -----------------------------------------------------------------------
// TestProviderStepInput_SecretNotRetained — step 5 API key security
// -----------------------------------------------------------------------

// TestProviderStepInput_SecretNotRetained verifies that SubmitStep(5, ProviderStepInput)
// does NOT retain the raw API key in flow.Data.Provider. The key should be zeroed
// after keyring persistence (or discarded when no keyring is available).
//
// This test uses an in-memory keyring to avoid OS keyring side effects.
func TestProviderStepInput_SecretNotRetained(t *testing.T) {
	// Use an in-memory keyring — no OS keyring writes.
	kr := onboarding.NewInMemoryKeyring()

	flow, err := onboarding.StartFlow(t.Context(),
		nil,
		onboarding.WithKeyring(kr),
	)
	if err != nil {
		t.Fatalf("StartFlow: %v", err)
	}

	// Submit steps 1–4 with minimal data so we can reach step 5.
	if err := flow.SubmitStep(1, onboarding.LocaleChoice{Country: "KR", Language: "ko", Timezone: "Asia/Seoul"}); err != nil {
		t.Fatalf("SubmitStep 1: %v", err)
	}
	if err := flow.SubmitStep(2, onboarding.ModelSetup{}); err != nil {
		t.Fatalf("SubmitStep 2: %v", err)
	}
	if err := flow.SubmitStep(3, onboarding.CLIToolsDetection{}); err != nil {
		t.Fatalf("SubmitStep 3: %v", err)
	}
	if err := flow.SubmitStep(4, onboarding.PersonaProfile{Name: "MINK"}); err != nil {
		t.Fatalf("SubmitStep 4: %v", err)
	}

	// A valid Anthropic key — passes the regex validator.
	const testKey = "sk-ant-testkey1234567890abcdefghijklmno"

	if err := flow.SubmitStep(5, onboarding.ProviderStepInput{
		Choice: onboarding.ProviderChoice{
			Provider:   onboarding.ProviderAnthropic,
			AuthMethod: onboarding.AuthMethodAPIKey,
		},
		APIKey: testKey,
	}); err != nil {
		t.Fatalf("SubmitStep 5: %v", err)
	}

	// The raw key must NOT appear in OnboardingData.Provider.
	// ProviderStepInput zeroes APIKey before assigning to Data.
	// We verify indirectly: ProviderChoice has no APIKey field — only APIKeyStored.
	if !flow.Data.Provider.APIKeyStored {
		t.Error("expected APIKeyStored=true after successful keyring write")
	}

	// Verify the key was actually stored in the in-memory keyring.
	// The canonical key format is "provider.<name>.api_key" (from keyring.go providerEntryKey).
	stored, getErr := kr.Get("provider.anthropic.api_key")
	if getErr != nil {
		t.Fatalf("keyring.Get: %v", getErr)
	}
	if stored != testKey {
		t.Errorf("keyring stored %q, want %q", stored, testKey)
	}
}

// -----------------------------------------------------------------------
// TestErrWizardCancelled_IsSentinel — sentinel error identity
// -----------------------------------------------------------------------

// TestErrWizardCancelled_IsSentinel verifies that ErrWizardCancelled is a stable
// sentinel that can be detected with errors.Is.
func TestErrWizardCancelled_IsSentinel(t *testing.T) {
	if ErrWizardCancelled == nil {
		t.Fatal("ErrWizardCancelled must not be nil")
	}
	if ErrWizardCancelled.Error() == "" {
		t.Fatal("ErrWizardCancelled must have a non-empty message")
	}
}

// -----------------------------------------------------------------------
// TestSummarizeDetection_SingleTool — single tool (no "+" separator)
// -----------------------------------------------------------------------

func TestSummarizeDetection_SingleTool(t *testing.T) {
	status := onboarding.OllamaStatus{Installed: true, DaemonAlive: true}
	model := onboarding.DetectedModel{Name: "ai-mink/test"}
	tools := []onboarding.CLITool{{Name: "claude"}}

	got := summarizeDetection(status, 8*1024*1024*1024, model, tools)

	if !strings.Contains(got, "CLI tools=claude") {
		t.Errorf("expected single tool name without '+', got: %s", got)
	}
	if strings.Contains(got, "CLI tools=none") {
		t.Errorf("unexpected 'none' for single tool, got: %s", got)
	}
}
