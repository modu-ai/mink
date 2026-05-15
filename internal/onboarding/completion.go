// Package onboarding — completion.go implements the onboarding completion config
// writer that persists the 7-step onboarding result to ~/.mink/config.yaml (global
// half) and ./.mink/config.yaml (project half).
//
// Design decision for the global config merge:
//
//	We load the existing global file (if any) into a map[string]any via yaml.Unmarshal,
//	mutate only the "model", "delegation", and "providers" top-level keys with
//	onboarding-collected data, and re-marshal the whole map. This approach was chosen
//	over struct-with-inline because yaml.v3 inline maps require the field to be
//	map[string]any *and* all other struct fields must be explicitly listed, which
//	creates a tight coupling between the writer and the on-disk schema.  The raw-map
//	approach is simpler, survives unknown keys from install.sh transparently, and
//	keeps the round-trip deterministic (yaml.v3 sorts map keys alphabetically).
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6.7
// REQ: REQ-OB-027
package onboarding

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Sentinel errors for completion config write operations.
// Callers use errors.Is to distinguish error categories.
var (
	// ErrCompletionMarshal is returned when yaml.Marshal fails.
	ErrCompletionMarshal = errors.New("completion: yaml marshal failed")

	// ErrCompletionWrite is returned when a file write operation fails.
	ErrCompletionWrite = errors.New("completion: file write failed")

	// ErrCompletionMerge is returned when loading or merging an existing config fails.
	ErrCompletionMerge = errors.New("completion: existing config merge failed")

	// ErrCompletionGlobalConfig is returned when writing the global config half fails.
	ErrCompletionGlobalConfig = errors.New("completion: global config write failed")

	// ErrCompletionProjectConfig is returned when writing the project config half fails.
	ErrCompletionProjectConfig = errors.New("completion: project config write failed")
)

// CompletionOptions allows callers and tests to override resolved paths and to
// enable a dry-run mode (marshal only, no disk I/O).
//
// All path overrides default to the canonical paths from the paths package when
// left empty.
type CompletionOptions struct {
	// DryRun skips all MkdirAll and WriteFile calls. Marshal errors are still
	// surfaced so callers can detect serialization issues without touching disk.
	DryRun bool

	// GlobalConfigPathOverride replaces paths.GlobalConfigPath() when non-empty.
	GlobalConfigPathOverride string

	// ProjectConfigPathOverride replaces paths.ProjectConfigPath() when non-empty.
	ProjectConfigPathOverride string

	// CompletedMarkerOverride replaces paths.OnboardingCompletedPath() when non-empty.
	CompletedMarkerOverride string

	// Now is called to obtain the current time for WriteOnboardingCompleted.
	// When nil, time.Now is used.
	Now func() time.Time
}

// WriteCompletionConfig writes the global and project halves of the onboarding
// result to disk.
//
// Global half (~/.mink/config.yaml):
//   - If the file already exists (e.g., pre-written by install.sh), the function
//     loads it into a raw map, merges only the "model", "delegation", and "providers"
//     keys with onboarding data taking precedence, and re-marshals the whole map
//     (unknown install.sh keys survive round-trip).
//   - If the file does not exist, the global half is written from scratch.
//
// Project half (./.mink/config.yaml):
//   - Always fully overwritten from data — no merge.
//
// The API key string is NEVER written to disk; only the source label
// ("keyring" | "env" | "none") is recorded under providers.<name>.api_key_source.
//
// @MX:ANCHOR: [AUTO] WriteCompletionConfig is the single writer for both config halves.
// @MX:REASON: All downstream consumers (Complete, CLI wiring Phase 1F, future re-config)
// call this function; signature changes propagate immediately to all callers.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6.7 REQ-OB-027
func WriteCompletionConfig(data *OnboardingData, kr KeyringClient, opts CompletionOptions) error {
	if err := writeGlobalConfig(data, kr, opts); err != nil {
		return fmt.Errorf("%w: %v", ErrCompletionGlobalConfig, err)
	}
	if err := writeProjectConfig(data, opts); err != nil {
		return fmt.Errorf("%w: %v", ErrCompletionProjectConfig, err)
	}
	return nil
}

// WriteOnboardingCompleted writes an RFC3339 timestamp to the onboarding marker
// file. If the marker already exists, the function preserves the original content
// and returns nil (idempotent — protects against double-fire from restart scenarios).
func WriteOnboardingCompleted(opts CompletionOptions) error {
	path, err := resolveMarkerPath(opts)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCompletionWrite, err)
	}

	if !opts.DryRun {
		// Idempotency: if the marker already exists, preserve it.
		if _, statErr := os.Stat(path); statErr == nil {
			return nil
		}
	}

	now := time.Now
	if opts.Now != nil {
		now = opts.Now
	}
	content := now().Format(time.RFC3339) + "\n"

	if opts.DryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("%w: mkdir %s: %v", ErrCompletionWrite, filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("%w: %v", ErrCompletionWrite, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// writeGlobalConfig builds and persists the global (~/.mink/config.yaml) half.
func writeGlobalConfig(data *OnboardingData, kr KeyringClient, opts CompletionOptions) error {
	path, err := resolveGlobalPath(opts)
	if err != nil {
		return err
	}

	// Load existing map (or start fresh).
	existing := make(map[string]any)
	if b, readErr := os.ReadFile(path); readErr == nil {
		if unmarshalErr := yaml.Unmarshal(b, &existing); unmarshalErr != nil {
			return fmt.Errorf("%w: %v", ErrCompletionMerge, unmarshalErr)
		}
	}

	// Overwrite model section.
	modelSection := buildGlobalModel(data)
	if modelSection != nil {
		existing["model"] = modelSection
	}

	// Overwrite delegation section.
	delegSection := buildGlobalDelegation(data)
	if delegSection != nil {
		existing["delegation"] = delegSection
	}

	// Build providers section — only when a concrete provider is selected.
	if data.Provider.Provider != ProviderUnset && data.Provider.Provider != "" {
		providerEntry, err := buildProviderEntry(data, kr)
		if err != nil {
			return err
		}
		// Merge: preserve any existing providers; current provider wins on conflict.
		existing["providers"] = mergeProviders(existing["providers"], string(data.Provider.Provider), providerEntry)
	}

	// Marshal back.
	b, err := yaml.Marshal(existing)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCompletionMarshal, err)
	}

	if opts.DryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("%w: %v", ErrCompletionWrite, err)
	}
	return nil
}

// writeProjectConfig builds and persists the project (./.mink/config.yaml) half.
// The project config is always a full overwrite — no merge.
func writeProjectConfig(data *OnboardingData, opts CompletionOptions) error {
	path, err := resolveProjectPath(opts)
	if err != nil {
		return err
	}

	honorific := string(data.Persona.HonorificLevel)
	if honorific == "" {
		honorific = "formal"
	}

	personaName := data.Persona.Name
	if personaName == "" {
		personaName = "User"
	}

	messengerType := string(data.Messenger.Type)
	if messengerType == "" {
		messengerType = "local_terminal"
	}

	proj := map[string]any{
		"persona": map[string]any{
			"name":            personaName,
			"honorific_level": honorific,
			"pronouns":        data.Persona.Pronouns,
		},
		"messenger": map[string]any{
			"type": messengerType,
		},
		"consent": map[string]any{
			"conversation_storage_local": data.Consent.ConversationStorageLocal,
			"lora_training":              data.Consent.LoRATrainingAllowed,
			"telemetry":                  data.Consent.TelemetryEnabled,
			"crash_reporting":            data.Consent.CrashReportingEnabled,
		},
	}

	// Remove empty pronouns to keep the file clean.
	if data.Persona.Pronouns == "" {
		persona := proj["persona"].(map[string]any)
		delete(persona, "pronouns")
	}

	b, err := yaml.Marshal(proj)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCompletionMarshal, err)
	}

	if opts.DryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("%w: %v", ErrCompletionWrite, err)
	}
	return nil
}

// buildGlobalModel constructs the "model" section map from ModelSetup.
//
// The model.provider field records the LOCAL execution engine (always "ollama" when
// Ollama is installed), distinct from the providers.* section which records the API
// provider. This matches the AC-OB-027 sample yaml where model.provider="ollama" and
// providers.anthropic records the cloud API key source.
//
// Returns nil when no model information is available.
func buildGlobalModel(data *OnboardingData) map[string]any {
	model := data.Model.SelectedModel
	if model == "" {
		model = data.Model.DetectedModel
	}
	if model == "" && !data.Model.OllamaInstalled {
		return nil
	}

	// model.provider is the local inference engine.
	// Use "ollama" when Ollama is installed, otherwise leave unset.
	engineProvider := ""
	if data.Model.OllamaInstalled {
		engineProvider = "ollama"
	}

	m := make(map[string]any)
	if model != "" {
		m["selected"] = model
	}
	if engineProvider != "" {
		m["provider"] = engineProvider
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

// buildGlobalDelegation constructs the "delegation" section map from CLIToolsDetection.
// Returns nil when no tools are detected.
func buildGlobalDelegation(data *OnboardingData) map[string]any {
	if len(data.CLITools.DetectedTools) == 0 {
		return nil
	}

	tools := make([]any, 0, len(data.CLITools.DetectedTools))
	for _, t := range data.CLITools.DetectedTools {
		entry := map[string]any{"name": t.Name}
		if t.Version != "" {
			entry["version"] = t.Version
		}
		tools = append(tools, entry)
	}
	return map[string]any{"available_tools": tools}
}

// buildProviderEntry resolves the api_key_source label for the selected provider.
// The API key string itself is never returned or stored.
func buildProviderEntry(data *OnboardingData, kr KeyringClient) (map[string]any, error) {
	// AuthMethodEnv always wins regardless of keyring state.
	if data.Provider.AuthMethod == AuthMethodEnv {
		return map[string]any{"api_key_source": "env"}, nil
	}

	// For nil keyring, fall back gracefully to "none".
	if kr == nil {
		return map[string]any{"api_key_source": "none"}, nil
	}

	// Probe keyring.
	_, err := GetProviderAPIKey(kr, string(data.Provider.Provider))
	if err == nil {
		return map[string]any{"api_key_source": "keyring"}, nil
	}
	if errors.Is(err, ErrKeyNotFound) || errors.Is(err, ErrNilKeyringClient) {
		return map[string]any{"api_key_source": "none"}, nil
	}
	// Any other keyring error propagates.
	return nil, err
}

// mergeProviders merges a single provider entry into the existing providers map.
// The new entry for the current provider always wins.
func mergeProviders(existing any, providerName string, entry map[string]any) map[string]any {
	result := make(map[string]any)
	if existingMap, ok := existing.(map[string]any); ok {
		maps.Copy(result, existingMap)
	}
	result[providerName] = entry
	return result
}

// resolveGlobalPath returns the effective global config path.
func resolveGlobalPath(opts CompletionOptions) (string, error) {
	if opts.GlobalConfigPathOverride != "" {
		return opts.GlobalConfigPathOverride, nil
	}
	return GlobalConfigPath()
}

// resolveProjectPath returns the effective project config path.
func resolveProjectPath(opts CompletionOptions) (string, error) {
	if opts.ProjectConfigPathOverride != "" {
		return opts.ProjectConfigPathOverride, nil
	}
	return ProjectConfigPath()
}

// resolveMarkerPath returns the effective onboarding-completed marker path.
func resolveMarkerPath(opts CompletionOptions) (string, error) {
	if opts.CompletedMarkerOverride != "" {
		return opts.CompletedMarkerOverride, nil
	}
	return OnboardingCompletedPath()
}
