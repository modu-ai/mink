// Package credential — DiscordCombo credential type.
//
// DiscordCombo holds the Ed25519 public key (for interaction request
// verification) and the bot token (for Discord API calls).  AppID is
// optional metadata that may be populated after registration.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (research.md §4.2, M3 T-013)
package credential

import (
	"fmt"
	"regexp"
)

// hexPattern matches a string that consists exclusively of lowercase
// hexadecimal characters and is exactly 64 characters long.
var hexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// DiscordCombo holds the Ed25519 public key and bot token for a Discord
// application.  It implements the Credential interface.
type DiscordCombo struct {
	// PublicKey is the Ed25519 public key in lowercase hex encoding (64 chars,
	// i.e. 32 bytes).  Used to verify the authenticity of interaction payloads
	// sent by Discord (required).
	PublicKey string

	// BotToken is the Discord bot token beginning with "Bot " (required).
	BotToken string

	// AppID is the Discord application ID (optional; may be empty on initial
	// registration).
	AppID string
}

// Kind returns KindDiscordCombo.
func (d DiscordCombo) Kind() Kind {
	return KindDiscordCombo
}

// MaskedString returns a log-safe representation using the last 4 characters
// of BotToken (the primary API credential).
func (d DiscordCombo) MaskedString() string {
	return MaskedString(d.BotToken)
}

// Validate checks that PublicKey (exactly 64 lowercase hex chars) and BotToken
// are both present and well-formed.
// AppID is optional and not validated.
func (d DiscordCombo) Validate() error {
	if d.PublicKey == "" {
		return fmt.Errorf("discord_combo: public_key is required: %w", ErrSchemaViolation)
	}
	if !hexPattern.MatchString(d.PublicKey) {
		return fmt.Errorf(
			"discord_combo: public_key must be exactly 64 lowercase hex characters: %w",
			ErrSchemaViolation,
		)
	}
	if d.BotToken == "" {
		return fmt.Errorf("discord_combo: bot_token is required: %w", ErrSchemaViolation)
	}
	return nil
}
