// Package credential — BotToken credential type.
//
// BotToken is used for the Telegram bot provider.  A single token string is
// sufficient for all Telegram Bot API calls.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (research.md §4.2, M3 T-013)
package credential

import "fmt"

// BotToken holds a single bot token string.  It implements the Credential
// interface.
type BotToken struct {
	// Provider is the logical provider identifier (e.g. "telegram_bot").
	Provider string

	// Token is the raw bot token (e.g. "123456:ABC-DEF...").
	Token string
}

// Kind returns KindBotToken.
func (b BotToken) Kind() Kind {
	return KindBotToken
}

// MaskedString returns a log-safe representation with the last 4 characters
// of Token visible.
func (b BotToken) MaskedString() string {
	return MaskedString(b.Token)
}

// Validate checks that both Provider and Token are non-empty.
// Returns a wrapped ErrSchemaViolation when either field is missing.
func (b BotToken) Validate() error {
	if b.Provider == "" {
		return fmt.Errorf("bot_token: provider is required: %w", ErrSchemaViolation)
	}
	if b.Token == "" {
		return fmt.Errorf("bot_token: token is required: %w", ErrSchemaViolation)
	}
	return nil
}
