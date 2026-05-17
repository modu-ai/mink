// Package credential — SlackCombo credential type.
//
// SlackCombo holds the Slack signing_secret + bot_token pair that is required
// for both interactive event verification (signing_secret) and API calls
// (bot_token).  AppID and TeamID are optional at registration time and may be
// populated later after the Slack OAuth app installation.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (research.md §4.2, M3 T-013)
package credential

import "fmt"

// SlackCombo holds the signing secret and bot token for a Slack application.
// It implements the Credential interface.
type SlackCombo struct {
	// SigningSecret is the Slack app signing secret used to verify request
	// authenticity (required).
	SigningSecret string

	// BotToken is the OAuth bot token beginning with "xoxb-" (required).
	BotToken string

	// AppID is the Slack application ID (optional; may be empty on initial
	// registration and populated after Slack OAuth install).
	AppID string

	// TeamID is the Slack workspace team ID (optional; populated after OAuth
	// install when the workspace is known).
	TeamID string
}

// Kind returns KindSlackCombo.
func (s SlackCombo) Kind() Kind {
	return KindSlackCombo
}

// MaskedString returns a log-safe representation using the last 4 characters
// of BotToken (the primary API credential).
func (s SlackCombo) MaskedString() string {
	return MaskedString(s.BotToken)
}

// Validate checks that the two required fields are present.
// AppID and TeamID are optional and not validated here.
func (s SlackCombo) Validate() error {
	if s.SigningSecret == "" {
		return fmt.Errorf("slack_combo: signing_secret is required: %w", ErrSchemaViolation)
	}
	if s.BotToken == "" {
		return fmt.Errorf("slack_combo: bot_token is required: %w", ErrSchemaViolation)
	}
	return nil
}
