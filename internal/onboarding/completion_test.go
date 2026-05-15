// Package onboarding — completion_test.go exercises WriteCompletionConfig and
// WriteOnboardingCompleted.
//
// All tests use t.TempDir() + t.Setenv for hermetic isolation; no host OS keyring
// or file system state is touched.
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6.7 AC-OB-027
package onboarding

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// newTestData returns a fully-populated OnboardingData suitable as a test fixture.
func newTestData() *OnboardingData {
	return &OnboardingData{
		Locale: LocaleChoice{
			Country:  "KR",
			Language: "ko",
			Timezone: "Asia/Seoul",
		},
		Model: ModelSetup{
			OllamaInstalled: true,
			DetectedModel:   "ai-mink/gemma4-e4b-rl-v1:q5_k_m",
			SelectedModel:   "ai-mink/gemma4-e4b-rl-v1:q5_k_m",
			ModelSizeBytes:  4_000_000_000,
			RAMBytes:        16_000_000_000,
		},
		CLITools: CLIToolsDetection{
			DetectedTools: []CLITool{
				{Name: "claude", Version: "1.2.3", Path: "/usr/local/bin/claude"},
				{Name: "codex", Version: "0.5.0", Path: "/usr/local/bin/codex"},
			},
		},
		Persona: PersonaProfile{
			Name:           "User",
			HonorificLevel: HonorificFormal,
			Pronouns:       "they/them",
		},
		Provider: ProviderChoice{
			Provider:     ProviderAnthropic,
			AuthMethod:   AuthMethodAPIKey,
			APIKeyStored: true,
		},
		Messenger: MessengerChannel{
			Type: MessengerLocalTerminal,
		},
		Consent: ConsentFlags{
			ConversationStorageLocal: true,
			LoRATrainingAllowed:      false,
			TelemetryEnabled:         false,
			CrashReportingEnabled:    false,
		},
	}
}

// tempOpts builds a CompletionOptions that writes inside t's temp directories.
func tempOpts(t *testing.T) CompletionOptions {
	t.Helper()
	globalDir := t.TempDir()
	projectDir := t.TempDir()
	markerDir := t.TempDir()
	return CompletionOptions{
		GlobalConfigPathOverride:  globalDir + "/config.yaml",
		ProjectConfigPathOverride: projectDir + "/config.yaml",
		CompletedMarkerOverride:   markerDir + "/onboarding-completed",
	}
}

// TestWriteCompletionConfig_BothFilesRoundTrip verifies that a full OnboardingData
// produces both config files with all expected fields.
func TestWriteCompletionConfig_BothFilesRoundTrip(t *testing.T) {
	opts := tempOpts(t)
	kr := NewInMemoryKeyring()
	_ = SetProviderAPIKey(kr, "anthropic", "sk-test-key")

	data := newTestData()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	// --- global file ---
	gBytes, err := os.ReadFile(opts.GlobalConfigPathOverride)
	if err != nil {
		t.Fatalf("read global: %v", err)
	}
	var g map[string]any
	if err := yaml.Unmarshal(gBytes, &g); err != nil {
		t.Fatalf("unmarshal global: %v", err)
	}
	model, ok := g["model"].(map[string]any)
	if !ok {
		t.Fatalf("global model section missing; got %v", g)
	}
	if model["selected"] != "ai-mink/gemma4-e4b-rl-v1:q5_k_m" {
		t.Errorf("model.selected: got %q", model["selected"])
	}
	if model["provider"] != "ollama" {
		t.Errorf("model.provider: got %q", model["provider"])
	}
	deleg, ok := g["delegation"].(map[string]any)
	if !ok {
		t.Fatalf("global delegation section missing; got %v", g)
	}
	tools, _ := deleg["available_tools"].([]any)
	if len(tools) != 2 {
		t.Errorf("delegation.available_tools: want 2, got %d", len(tools))
	}

	// --- project file ---
	pBytes, err := os.ReadFile(opts.ProjectConfigPathOverride)
	if err != nil {
		t.Fatalf("read project: %v", err)
	}
	var p map[string]any
	if err := yaml.Unmarshal(pBytes, &p); err != nil {
		t.Fatalf("unmarshal project: %v", err)
	}
	persona, ok := p["persona"].(map[string]any)
	if !ok {
		t.Fatalf("project persona section missing; got %v", p)
	}
	if persona["name"] != "User" {
		t.Errorf("persona.name: got %q", persona["name"])
	}
	if persona["honorific_level"] != "formal" {
		t.Errorf("persona.honorific_level: got %q", persona["honorific_level"])
	}
	messenger, ok := p["messenger"].(map[string]any)
	if !ok {
		t.Fatalf("project messenger section missing")
	}
	if messenger["type"] != "local_terminal" {
		t.Errorf("messenger.type: got %q", messenger["type"])
	}
	consent, ok := p["consent"].(map[string]any)
	if !ok {
		t.Fatalf("project consent section missing")
	}
	if consent["conversation_storage_local"] != true {
		t.Errorf("consent.conversation_storage_local: got %v", consent["conversation_storage_local"])
	}
}

// TestWriteCompletionConfig_GlobalMergeWithInstallSh verifies that a pre-existing
// global config is merged and that unknown keys ("installed_at") survive round-trip.
func TestWriteCompletionConfig_GlobalMergeWithInstallSh(t *testing.T) {
	opts := tempOpts(t)

	// Pre-write a global config as if install.sh had created it.
	preExisting := map[string]any{
		"model": map[string]any{
			"selected": "old-model",
			"provider": "ollama",
		},
		"delegation": map[string]any{
			"available_tools": []any{
				map[string]any{"name": "stale-tool", "version": "0.0.1"},
			},
		},
		"installed_at": "2026-05-15T10:00:00Z",
	}
	preBytes, _ := yaml.Marshal(preExisting)
	_ = os.WriteFile(opts.GlobalConfigPathOverride, preBytes, 0o600)

	data := newTestData()
	data.Model.SelectedModel = "ai-mink/new-model"
	data.Provider.Provider = ProviderUnset // no providers section expected

	kr := NewInMemoryKeyring()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	gBytes, err := os.ReadFile(opts.GlobalConfigPathOverride)
	if err != nil {
		t.Fatalf("read global: %v", err)
	}
	var g map[string]any
	_ = yaml.Unmarshal(gBytes, &g)

	// (a) onboarding model wins
	model := g["model"].(map[string]any)
	if model["selected"] != "ai-mink/new-model" {
		t.Errorf("model.selected: want new-model, got %v", model["selected"])
	}

	// (b) installed_at survives round-trip
	if g["installed_at"] != "2026-05-15T10:00:00Z" {
		t.Errorf("installed_at missing or wrong: %v", g["installed_at"])
	}

	// (c) new tools section replaces old
	deleg := g["delegation"].(map[string]any)
	tools := deleg["available_tools"].([]any)
	for _, raw := range tools {
		tool := raw.(map[string]any)
		if tool["name"] == "stale-tool" {
			t.Errorf("stale-tool should have been replaced")
		}
	}
}

// TestWriteCompletionConfig_ProjectHalfOverwrites verifies that a pre-existing
// project config is fully replaced.
func TestWriteCompletionConfig_ProjectHalfOverwrites(t *testing.T) {
	opts := tempOpts(t)

	stale := map[string]any{
		"persona": map[string]any{
			"name":            "OldName",
			"honorific_level": "casual",
		},
		"messenger": map[string]any{"type": "slack"},
		"consent": map[string]any{
			"conversation_storage_local": false,
			"lora_training":              true,
			"telemetry":                  true,
			"crash_reporting":            true,
		},
	}
	staleBytes, _ := yaml.Marshal(stale)
	_ = os.WriteFile(opts.ProjectConfigPathOverride, staleBytes, 0o600)

	data := newTestData()
	data.Persona.Name = "NewName"
	data.Persona.HonorificLevel = HonorificIntimate
	data.Messenger.Type = MessengerTelegram

	kr := NewInMemoryKeyring()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	pBytes, _ := os.ReadFile(opts.ProjectConfigPathOverride)
	var p map[string]any
	_ = yaml.Unmarshal(pBytes, &p)

	persona := p["persona"].(map[string]any)
	if persona["name"] != "NewName" {
		t.Errorf("persona.name: got %v", persona["name"])
	}
	if persona["honorific_level"] != "intimate" {
		t.Errorf("persona.honorific_level: got %v", persona["honorific_level"])
	}
	messenger := p["messenger"].(map[string]any)
	if messenger["type"] != "telegram" {
		t.Errorf("messenger.type: got %v", messenger["type"])
	}
	consent := p["consent"].(map[string]any)
	if consent["lora_training"] != false {
		t.Errorf("lora_training should be false")
	}
}

// TestWriteCompletionConfig_ApiKeySource_Keyring verifies that when the keyring
// has the API key, the output shows "keyring" and the secret is never written to disk.
func TestWriteCompletionConfig_ApiKeySource_Keyring(t *testing.T) {
	opts := tempOpts(t)
	kr := NewInMemoryKeyring()
	_ = SetProviderAPIKey(kr, "anthropic", "sk-secret-value")

	data := newTestData()
	data.Provider.Provider = ProviderAnthropic
	data.Provider.AuthMethod = AuthMethodAPIKey

	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	gBytes, _ := os.ReadFile(opts.GlobalConfigPathOverride)

	// api_key_source must be "keyring"
	var g map[string]any
	_ = yaml.Unmarshal(gBytes, &g)
	providers := g["providers"].(map[string]any)
	anthropic := providers["anthropic"].(map[string]any)
	if anthropic["api_key_source"] != "keyring" {
		t.Errorf("api_key_source: want keyring, got %v", anthropic["api_key_source"])
	}

	// Defense-in-depth: secret string must NOT appear in any written file.
	allFiles := []string{opts.GlobalConfigPathOverride, opts.ProjectConfigPathOverride}
	for _, f := range allFiles {
		b, _ := os.ReadFile(f)
		if strings.Contains(string(b), "sk-secret-value") {
			t.Errorf("secret leaked in %s", f)
		}
	}
}

// TestWriteCompletionConfig_ApiKeySource_None verifies that an absent keyring entry
// produces api_key_source: none.
func TestWriteCompletionConfig_ApiKeySource_None(t *testing.T) {
	opts := tempOpts(t)
	kr := NewInMemoryKeyring() // empty

	data := newTestData()
	data.Provider.Provider = ProviderAnthropic
	data.Provider.AuthMethod = AuthMethodAPIKey

	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	gBytes, _ := os.ReadFile(opts.GlobalConfigPathOverride)
	var g map[string]any
	_ = yaml.Unmarshal(gBytes, &g)
	providers := g["providers"].(map[string]any)
	anthropic := providers["anthropic"].(map[string]any)
	if anthropic["api_key_source"] != "none" {
		t.Errorf("api_key_source: want none, got %v", anthropic["api_key_source"])
	}
}

// TestWriteCompletionConfig_ApiKeySource_Env verifies that AuthMethodEnv overrides
// keyring lookup and produces api_key_source: env.
func TestWriteCompletionConfig_ApiKeySource_Env(t *testing.T) {
	opts := tempOpts(t)
	// Even if keyring has the key, env auth method should win.
	kr := NewInMemoryKeyring()
	_ = SetProviderAPIKey(kr, "anthropic", "sk-irrelevant")

	data := newTestData()
	data.Provider.Provider = ProviderAnthropic
	data.Provider.AuthMethod = AuthMethodEnv

	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	gBytes, _ := os.ReadFile(opts.GlobalConfigPathOverride)
	var g map[string]any
	_ = yaml.Unmarshal(gBytes, &g)
	providers := g["providers"].(map[string]any)
	anthropic := providers["anthropic"].(map[string]any)
	if anthropic["api_key_source"] != "env" {
		t.Errorf("api_key_source: want env, got %v", anthropic["api_key_source"])
	}
}

// TestWriteCompletionConfig_NilKeyringFallsBackToNone verifies graceful handling
// when nil is passed as KeyringClient — no panic, api_key_source: none.
func TestWriteCompletionConfig_NilKeyringFallsBackToNone(t *testing.T) {
	opts := tempOpts(t)
	data := newTestData()
	data.Provider.Provider = ProviderAnthropic
	data.Provider.AuthMethod = AuthMethodAPIKey

	if err := WriteCompletionConfig(data, nil, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	gBytes, _ := os.ReadFile(opts.GlobalConfigPathOverride)
	var g map[string]any
	_ = yaml.Unmarshal(gBytes, &g)
	providers := g["providers"].(map[string]any)
	anthropic := providers["anthropic"].(map[string]any)
	if anthropic["api_key_source"] != "none" {
		t.Errorf("api_key_source: want none, got %v", anthropic["api_key_source"])
	}
}

// TestWriteCompletionConfig_ProviderUnsetSkipsProvidersSection verifies that
// ProviderUnset results in no "providers" key in the global config.
func TestWriteCompletionConfig_ProviderUnsetSkipsProvidersSection(t *testing.T) {
	opts := tempOpts(t)
	data := newTestData()
	data.Provider.Provider = ProviderUnset

	kr := NewInMemoryKeyring()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	gBytes, _ := os.ReadFile(opts.GlobalConfigPathOverride)
	var g map[string]any
	_ = yaml.Unmarshal(gBytes, &g)
	if _, found := g["providers"]; found {
		t.Errorf("providers section should be absent when Provider == ProviderUnset")
	}
}

// TestWriteCompletionConfig_DryRun verifies that DryRun=true writes no files to disk.
func TestWriteCompletionConfig_DryRun(t *testing.T) {
	opts := tempOpts(t)
	opts.DryRun = true

	data := newTestData()
	kr := NewInMemoryKeyring()

	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	for _, path := range []string{opts.GlobalConfigPathOverride, opts.ProjectConfigPathOverride} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("DryRun: file should not exist: %s (stat err=%v)", path, err)
		}
	}
}

// TestWriteCompletionConfig_FileModeIs0600 verifies that both config files are
// written with permission 0600.
func TestWriteCompletionConfig_FileModeIs0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("skipping file permission test on Windows")
	}
	opts := tempOpts(t)
	kr := NewInMemoryKeyring()
	_ = SetProviderAPIKey(kr, "anthropic", "sk-any")

	data := newTestData()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	for _, path := range []string{opts.GlobalConfigPathOverride, opts.ProjectConfigPathOverride} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", path, err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s: want mode 0600, got %04o", path, perm)
		}
	}
}

// TestWriteOnboardingCompleted_CreatesTimestamp verifies that the marker file
// contains an RFC3339 timestamp equal to the injected Now function's return.
func TestWriteOnboardingCompleted_CreatesTimestamp(t *testing.T) {
	opts := tempOpts(t)
	fixed := time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC)
	opts.Now = func() time.Time { return fixed }

	if err := WriteOnboardingCompleted(opts); err != nil {
		t.Fatalf("WriteOnboardingCompleted: %v", err)
	}

	b, err := os.ReadFile(opts.CompletedMarkerOverride)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	content := strings.TrimSpace(string(b))
	parsed, err := time.Parse(time.RFC3339, content)
	if err != nil {
		t.Fatalf("parse RFC3339: %v (content=%q)", err, content)
	}
	if !parsed.Equal(fixed) {
		t.Errorf("want %v, got %v", fixed, parsed)
	}
}

// TestWriteOnboardingCompleted_Idempotent verifies that calling WriteOnboardingCompleted
// twice keeps the FIRST timestamp, not the second.
func TestWriteOnboardingCompleted_Idempotent(t *testing.T) {
	opts := tempOpts(t)
	first := time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC)
	second := time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC)

	opts.Now = func() time.Time { return first }
	if err := WriteOnboardingCompleted(opts); err != nil {
		t.Fatalf("first call: %v", err)
	}

	opts.Now = func() time.Time { return second }
	if err := WriteOnboardingCompleted(opts); err != nil {
		t.Fatalf("second call: %v", err)
	}

	b, _ := os.ReadFile(opts.CompletedMarkerOverride)
	content := strings.TrimSpace(string(b))
	parsed, _ := time.Parse(time.RFC3339, content)
	if !parsed.Equal(first) {
		t.Errorf("idempotent: want first timestamp %v, got %v", first, parsed)
	}
}

// TestWriteCompletionConfig_HonorificDefaultsFormal verifies that an empty
// HonorificLevel defaults to "formal" in the output yaml.
func TestWriteCompletionConfig_HonorificDefaultsFormal(t *testing.T) {
	opts := tempOpts(t)
	data := newTestData()
	data.Persona.HonorificLevel = "" // explicitly unset

	kr := NewInMemoryKeyring()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	pBytes, _ := os.ReadFile(opts.ProjectConfigPathOverride)
	var p map[string]any
	_ = yaml.Unmarshal(pBytes, &p)
	persona := p["persona"].(map[string]any)
	if persona["honorific_level"] != "formal" {
		t.Errorf("want formal, got %v", persona["honorific_level"])
	}
}

// TestWriteCompletionConfig_MessengerDefaultsLocalTerminal verifies that an empty
// Messenger.Type defaults to "local_terminal".
func TestWriteCompletionConfig_MessengerDefaultsLocalTerminal(t *testing.T) {
	opts := tempOpts(t)
	data := newTestData()
	data.Messenger.Type = "" // explicitly unset

	kr := NewInMemoryKeyring()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	pBytes, _ := os.ReadFile(opts.ProjectConfigPathOverride)
	var p map[string]any
	_ = yaml.Unmarshal(pBytes, &p)
	messenger := p["messenger"].(map[string]any)
	if messenger["type"] != "local_terminal" {
		t.Errorf("want local_terminal, got %v", messenger["type"])
	}
}

// TestWriteCompletionConfig_GlobalDirCreation verifies that a missing global config
// directory is created (MkdirAll) before writing.
func TestWriteCompletionConfig_GlobalDirCreation(t *testing.T) {
	base := t.TempDir()
	nestedPath := base + "/deeply/nested/.mink/config.yaml"
	projectDir := t.TempDir()
	markerDir := t.TempDir()

	opts := CompletionOptions{
		GlobalConfigPathOverride:  nestedPath,
		ProjectConfigPathOverride: projectDir + "/config.yaml",
		CompletedMarkerOverride:   markerDir + "/onboarding-completed",
	}

	kr := NewInMemoryKeyring()
	data := newTestData()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	if _, err := os.Stat(nestedPath); err != nil {
		t.Errorf("global config not created at nested path: %v", err)
	}
}

// TestWriteCompletionConfig_ErrorWrappingForUnwritableDir verifies that a write
// failure returns an error satisfying errors.Is(err, ErrCompletionGlobalConfig).
func TestWriteCompletionConfig_ErrorWrappingForUnwritableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("skipping chmod-based test on Windows")
	}

	base := t.TempDir()
	// Make the directory unwritable so MkdirAll of a subdirectory will fail.
	if err := os.Chmod(base, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(base, 0o755) })

	opts := CompletionOptions{
		GlobalConfigPathOverride:  base + "/sub/config.yaml",
		ProjectConfigPathOverride: t.TempDir() + "/config.yaml",
		CompletedMarkerOverride:   t.TempDir() + "/onboarding-completed",
	}

	kr := NewInMemoryKeyring()
	data := newTestData()
	err := WriteCompletionConfig(data, kr, opts)
	if err == nil {
		t.Fatal("expected error writing to unwritable dir, got nil")
	}
	if !errors.Is(err, ErrCompletionGlobalConfig) {
		t.Errorf("want errors.Is(err, ErrCompletionGlobalConfig), got %v", err)
	}
}

// TestWriteCompletionConfig_PersonaNameDefaultsUser verifies that an empty
// Persona.Name defaults to "User".
func TestWriteCompletionConfig_PersonaNameDefaultsUser(t *testing.T) {
	opts := tempOpts(t)
	data := newTestData()
	data.Persona.Name = "" // explicitly unset

	kr := NewInMemoryKeyring()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	pBytes, _ := os.ReadFile(opts.ProjectConfigPathOverride)
	var p map[string]any
	_ = yaml.Unmarshal(pBytes, &p)
	persona := p["persona"].(map[string]any)
	if persona["name"] != "User" {
		t.Errorf("want User, got %v", persona["name"])
	}
}

// TestWriteCompletionConfig_ToolsFromCLIDetection verifies that detected tools are
// written to the global delegation section in order.
func TestWriteCompletionConfig_ToolsFromCLIDetection(t *testing.T) {
	opts := tempOpts(t)
	data := newTestData()
	data.CLITools.DetectedTools = []CLITool{
		{Name: "gemini", Version: "2.0.0", Path: "/usr/bin/gemini"},
	}
	data.Provider.Provider = ProviderUnset

	kr := NewInMemoryKeyring()
	if err := WriteCompletionConfig(data, kr, opts); err != nil {
		t.Fatalf("WriteCompletionConfig: %v", err)
	}

	gBytes, _ := os.ReadFile(opts.GlobalConfigPathOverride)
	var g map[string]any
	_ = yaml.Unmarshal(gBytes, &g)
	deleg := g["delegation"].(map[string]any)
	tools := deleg["available_tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("want 1 tool, got %d", len(tools))
	}
	tool := tools[0].(map[string]any)
	if tool["name"] != "gemini" {
		t.Errorf("tool name: got %v", tool["name"])
	}
	if tool["version"] != "2.0.0" {
		t.Errorf("tool version: got %v", tool["version"])
	}
}

// TestWriteOnboardingCompleted_DryRun verifies DryRun prevents marker file creation.
func TestWriteOnboardingCompleted_DryRun(t *testing.T) {
	opts := tempOpts(t)
	opts.DryRun = true
	opts.Now = func() time.Time { return time.Now() }

	if err := WriteOnboardingCompleted(opts); err != nil {
		t.Fatalf("WriteOnboardingCompleted DryRun: %v", err)
	}

	if _, err := os.Stat(opts.CompletedMarkerOverride); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("DryRun: marker file should not exist")
	}
}
