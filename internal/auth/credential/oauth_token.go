// Package credential — OAuthToken credential type.
//
// OAuthToken is used for the Codex (ChatGPT) provider which uses OAuth 2.1
// + PKCE flow.  The access_token has a short TTL (60 min); the refresh_token
// enables silent renewal up to 8 days idle (research.md §3).
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (research.md §4.2, M3 T-013)
package credential

import (
	"fmt"
	"time"
)

// OAuthToken holds the OAuth 2.1 access + refresh token pair together with
// expiry metadata.  It implements the Credential interface.
type OAuthToken struct {
	// Provider is the logical provider identifier (e.g. "codex").
	Provider string

	// AccessToken is the short-lived bearer token sent to the LLM API.
	AccessToken string

	// RefreshToken is the long-lived token used to obtain a new AccessToken.
	// OpenAI may rotate the RefreshToken on each refresh call (research.md §3.4).
	RefreshToken string

	// ExpiresAt is the absolute UTC time at which AccessToken becomes invalid.
	// A zero value means the expiry is unknown and the token must be treated as
	// expired.
	ExpiresAt time.Time

	// Scope is the space-separated OAuth scope string returned by the
	// authorization server (e.g. "openid email profile offline_access").
	Scope string
}

// Kind returns KindOAuth.
func (o OAuthToken) Kind() Kind {
	return KindOAuth
}

// MaskedString returns a log-safe representation.  The AccessToken's last 4
// characters are revealed; everything else is masked with "***".
func (o OAuthToken) MaskedString() string {
	return MaskedString(o.AccessToken)
}

// Validate checks that all required fields are present.
// Returns a wrapped ErrSchemaViolation for each missing required field.
func (o OAuthToken) Validate() error {
	if o.Provider == "" {
		return fmt.Errorf("oauth: provider is required: %w", ErrSchemaViolation)
	}
	if o.AccessToken == "" {
		return fmt.Errorf("oauth: access_token is required: %w", ErrSchemaViolation)
	}
	if o.RefreshToken == "" {
		return fmt.Errorf("oauth: refresh_token is required: %w", ErrSchemaViolation)
	}
	if o.ExpiresAt.IsZero() {
		return fmt.Errorf("oauth: expires_at is required: %w", ErrSchemaViolation)
	}
	return nil
}
