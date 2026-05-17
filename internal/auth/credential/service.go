// Package credential defines the core Service interface and types for MINK's
// credential storage subsystem.
//
// The Service interface is the single point of interaction for all credential
// operations. Backends (keyring, file) implement this interface so that the
// dispatch layer can route transparently without callers caring which backend
// is active.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UB-1, UB-7, UB-8)
package credential

// Kind identifies the category of a stored credential.
// Only the five values defined below are valid; the schema validation in
// Credential.Validate() enforces this set (UB-6).
type Kind string

const (
	// KindAPIKey represents a simple long-lived API key (Anthropic / DeepSeek /
	// OpenAI GPT / z.ai GLM).
	KindAPIKey Kind = "api_key"

	// KindOAuth represents an OAuth 2.1 access + refresh token pair (Codex /
	// ChatGPT). The access_token has a short TTL; the refresh_token enables
	// silent renewal.
	KindOAuth Kind = "oauth"

	// KindBotToken represents a single bot token (Telegram).
	KindBotToken Kind = "bot_token"

	// KindSlackCombo represents the Slack signing_secret + bot_token pair.
	KindSlackCombo Kind = "slack_combo"

	// KindDiscordCombo represents the Discord Ed25519 public_key + bot_token pair.
	KindDiscordCombo Kind = "discord_combo"
)

// Credential is the common interface that every concrete credential type must
// satisfy. Backends store and retrieve opaque Credential values; consumers
// (e.g. LLM-ROUTING-V2) type-assert to the concrete type they expect.
type Credential interface {
	// Kind returns the credential category.
	Kind() Kind

	// MaskedString returns a safe log-friendly representation. Implementations
	// MUST NOT include the full plaintext value (UN-1).
	MaskedString() string

	// Validate checks that the credential payload is complete and schema-
	// compliant (UB-6). Returns ErrSchemaViolation if validation fails.
	Validate() error
}

// HealthStatus describes the storage state of a single provider's credential
// without leaking plaintext (UB-8).
type HealthStatus struct {
	// Present is true when a credential entry exists for the provider.
	Present bool

	// MaskedLast4 holds the last-4 representation (e.g. "***7890") of the
	// primary secret field. Empty string when Present is false.
	MaskedLast4 string

	// Backend identifies which storage backend returned this status
	// ("keyring" or "file").
	Backend string
}

// Service is the single abstraction for all credential lifecycle operations.
// Implementations must be safe for concurrent use.
//
// @MX:ANCHOR: [AUTO] Service is the primary interface consumed by keyring,
// file, and dispatch backends (fan_in >= 3).
// @MX:REASON: Any change to this interface signature propagates to all
// three backend implementations and their tests.
// @MX:SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UB-1)
type Service interface {
	// Store saves the credential for the given provider, overwriting any
	// existing entry (UB-4 single-account policy).
	Store(provider string, cred Credential) error

	// Load retrieves the credential for the given provider.
	// Returns ErrNotFound when no credential exists for the provider.
	Load(provider string) (Credential, error)

	// Delete removes the credential for the given provider.
	// Delete is idempotent: it returns nil when the entry is already absent
	// (ED-3).
	Delete(provider string) error

	// List returns the provider IDs that have a stored credential.
	// The returned slice contains provider identifiers only, never plaintext
	// values (UN-1).
	List() ([]string, error)

	// Health probes the presence and validity of the credential for the given
	// provider without leaking plaintext (UB-8).
	Health(provider string) (HealthStatus, error)
}
