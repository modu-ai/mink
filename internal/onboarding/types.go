// Package onboarding implements the 7-Step backend state machine for the MINK install wizard.
// This package is shared by both the CLI (charmbracelet/huh TUI) and the Web UI (React + Go HTTP)
// onboarding paths. It contains no file I/O, no OS keyring access, and no CLI/Web layer code.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
// REQ: REQ-OB-001, REQ-OB-024
package onboarding

// HonorificLevel describes the preferred address style toward the MINK persona.
// Values align with SPEC-MINK-ONBOARDING-001 §6.2 PersonaProfile.HonorificLevel.
type HonorificLevel string

const (
	// HonorificFormal is the polite/formal register (e.g., Korean 존댓말).
	HonorificFormal HonorificLevel = "formal"
	// HonorificCasual is the everyday informal register.
	HonorificCasual HonorificLevel = "casual"
	// HonorificIntimate is the close-friend / family register.
	HonorificIntimate HonorificLevel = "intimate"
)

// AuthMethod enumerates the supported LLM provider authentication strategies.
// Values align with SPEC-MINK-ONBOARDING-001 §6.2 ProviderChoice.AuthMethod.
type AuthMethod string

const (
	// AuthMethodOAuth is OAuth 2.0 browser-redirect flow (e.g., Google).
	AuthMethodOAuth AuthMethod = "oauth"
	// AuthMethodAPIKey is a bare API key stored in the OS keyring after onboarding.
	AuthMethodAPIKey AuthMethod = "api_key"
	// AuthMethodEnv reads the key from an environment variable at runtime.
	AuthMethodEnv AuthMethod = "env"
)

// Provider enumerates the known LLM provider identifiers.
// Values align with SPEC-MINK-ONBOARDING-001 §6.2 ProviderChoice.Provider.
type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
	ProviderGoogle    Provider = "google"
	ProviderOllama    Provider = "ollama"
	ProviderDeepSeek  Provider = "deepseek"
	ProviderCustom    Provider = "custom"
	// ProviderUnset is the sentinel for "no provider chosen yet" (Skip path).
	ProviderUnset Provider = "unset"
)

// MessengerType enumerates the supported first-channel messenger kinds.
// Values align with SPEC-MINK-ONBOARDING-001 §6.2 MessengerChannel.Type.
type MessengerType string

const (
	// MessengerLocalTerminal is the default embedded terminal channel.
	MessengerLocalTerminal MessengerType = "local_terminal"
	MessengerSlack         MessengerType = "slack"
	MessengerTelegram      MessengerType = "telegram"
	MessengerDiscord       MessengerType = "discord"
	MessengerCustom        MessengerType = "custom"
)

// LocaleChoice captures the user's locale selection from Step 1 (Welcome + Locale).
// @MX:NOTE: [AUTO] Fields derived from LOCALE-001 contract — fuller schema (Detect() result,
// CulturalContext, legal_flags) integrated in Phase 1B/2 when LOCALE-001 is wired in.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6.2 / SPEC-GOOSE-LOCALE-001
type LocaleChoice struct {
	Country    string   // ISO 3166-1 alpha-2, e.g., "KR", "DE"
	Language   string   // BCP 47 primary language tag, e.g., "ko", "fr"
	Timezone   string   // IANA timezone ID, e.g., "Asia/Seoul"
	LegalFlags []string // active legal-regime flags, e.g., ["GDPR", "UK_GDPR"]
}

// ModelSetup captures the Ollama and local-model state from Step 2 (Model Setup).
// Detection logic (Ollama binary probe, RAM measurement, model download) lives in
// model_setup.go — Phase 1D. For Phase 1A the struct is zero-valued on Skip.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 (v0.3 NEW)
type ModelSetup struct {
	OllamaInstalled bool   // true if `ollama` binary detected in PATH
	DetectedModel   string // e.g., "ai-mink/gemma4-e4b-rl-v1:q5_k_m"
	SelectedModel   string // user's final choice; defaults to DetectedModel
	ModelSizeBytes  int64  // estimated download size in bytes
	RAMBytes        int64  // detected system RAM in bytes
}

// CLIToolsDetection captures the PATH scan result from Step 3 (CLI Tools).
// Detection logic (command -v, --version parsing) lives in cli_detection.go — Phase 1D.
// For Phase 1A the struct holds whatever data the caller supplies (or zero on Skip).
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 (v0.3 NEW)
type CLIToolsDetection struct {
	DetectedTools []CLITool // tools found in PATH, empty when none detected
}

// CLITool represents a single CLI delegation target detected on the host.
type CLITool struct {
	Name    string // "claude" | "gemini" | "codex"
	Version string // parsed from `<tool> --version` stdout
	Path    string // absolute path to binary, e.g., "/usr/local/bin/claude"
}

// PersonaProfile captures the user's MINK persona configuration from Step 4 (Persona).
// Name validation (length, injection sanitization) is enforced in validators.go — Phase 1B.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
type PersonaProfile struct {
	Name           string         // required; 1..500 chars after sanitization (Phase 1B)
	HonorificLevel HonorificLevel // defaults to HonorificFormal when skipped
	Pronouns       string         // optional free-text
	SoulMarkdown   string         // body of the generated soul.md file
}

// ProviderChoice captures the LLM provider selection from Step 5 (Provider).
// API key storage into the OS keyring is performed in keyring.go — Phase 1C.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
type ProviderChoice struct {
	Provider       Provider   // selected provider, defaults to ProviderUnset when skipped
	AuthMethod     AuthMethod // authentication strategy chosen by the user
	APIKeyStored   bool       // true once the key is safely in the OS keyring (Phase 1C)
	CustomEndpoint string     // non-empty only when Provider == ProviderCustom
	PreferredModel string     // optional model name override within the provider
}

// MessengerChannel captures the first messaging channel from Step 6 (Messenger Channel).
// Default on Skip is Type = MessengerLocalTerminal.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
type MessengerChannel struct {
	Type        MessengerType // selected channel; default local_terminal
	BotTokenKey string        // keyring entry key set after token storage (Phase 1C)
}

// ConsentFlags captures user privacy and consent choices from Step 7 (Privacy & Consent).
// GDPR enforcement (Skip disallowed for EU users) is added in validators.go — Phase 1B.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2
type ConsentFlags struct {
	ConversationStorageLocal bool  // true = store conversations locally only; default true
	LoRATrainingAllowed      bool  // true = opt-in to LoRA fine-tuning; default false
	TelemetryEnabled         bool  // true = anonymous usage telemetry; default false
	CrashReportingEnabled    bool  // true = crash report upload; default false
	GDPRExplicitConsent      *bool // non-nil for EU/UK users; nil elsewhere (Phase 1B enforces)
}

// OnboardingData holds all data collected across the 7 onboarding steps.
// Fields are zero-valued until the corresponding step is submitted or populated by StartFlow.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 (v0.3 — 7-step layout)
// @MX:ANCHOR: [AUTO] Central data envelope shared by StartFlow, SubmitStep, Complete,
// CLI TUI, and Web UI HTTP handler — fan_in >= 5.
// @MX:REASON: Any field rename or layout change breaks all callers simultaneously.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 §6.2
type OnboardingData struct {
	Locale    LocaleChoice      // Step 1 — Welcome + Locale
	Model     ModelSetup        // Step 2 — Model Setup (v0.3 NEW)
	CLITools  CLIToolsDetection // Step 3 — CLI Tools (v0.3 NEW)
	Persona   PersonaProfile    // Step 4 — Persona (was Step 2 in v0.2)
	Provider  ProviderChoice    // Step 5 — Provider (was Step 3 in v0.2)
	Messenger MessengerChannel  // Step 6 — Messenger Channel (was Step 4 in v0.2)
	Consent   ConsentFlags      // Step 7 — Privacy & Consent (was Step 5 in v0.2)
}
