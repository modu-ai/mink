// Package credential — APIKey credential type.
//
// APIKey is used for Anthropic, DeepSeek, OpenAI GPT, and z.ai GLM providers.
// Additional concrete credential types (OAuthToken, BotToken, SlackCombo,
// DiscordCombo) will be added in M3 T-013.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (research.md §4.2)
package credential

import "fmt"

// APIKey holds a single long-lived API key value.
// It implements the Credential interface.
type APIKey struct {
	// Value is the raw API key string.  It must be non-empty to pass
	// Validate().
	Value string
}

// Kind returns KindAPIKey.
func (a APIKey) Kind() Kind {
	return KindAPIKey
}

// MaskedString returns a log-safe representation using the global masking
// helper so that plaintext is never written to logs (UN-1).
func (a APIKey) MaskedString() string {
	return MaskedString(a.Value)
}

// Validate checks that the APIKey payload is complete.
// Returns a wrapped ErrSchemaViolation when Value is empty.
func (a APIKey) Validate() error {
	if a.Value == "" {
		return fmt.Errorf("api_key: value is required: %w", ErrSchemaViolation)
	}
	return nil
}
