// Package onboarding — validators_test.go contains table-driven unit tests for all
// five validator functions in validators.go. Tests use stdlib testing only (no third-party).
// Each test function covers happy path, edge cases, and error path per SPEC §6.8.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.8 #2, #5, #6, #9, #12
package onboarding

import (
	"errors"
	"strings"
	"testing"
)

// ptrBool is a helper that returns a pointer to the given bool value.
func ptrBool(b bool) *bool {
	return &b
}

// TestValidatePersonaName covers AC-OB-005 (empty), AC-OB-015 (injection), and
// REQ-OB-017 (>500 bytes).
// SPEC §6.8 #2 (EmptyName), #9 (NameInjection)
func TestValidatePersonaName(t *testing.T) {
	// Build a 501-byte valid UTF-8 string (all ASCII 'a').
	longName := strings.Repeat("a", 501)

	// Build a valid Korean name well within the byte limit.
	koreanName := "김상호" // 9 bytes in UTF-8 (3 bytes per rune × 3 runes)

	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "empty string",
			input:   "",
			wantErr: ErrNameEmpty,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: ErrNameEmpty,
		},
		{
			name:    "tab and newline whitespace",
			input:   "\t\n",
			wantErr: ErrNameEmpty,
		},
		{
			name:    "501 byte ASCII name",
			input:   longName,
			wantErr: ErrNameTooLong,
		},
		{
			name:    "HTML injection with script tag",
			input:   "Hacker<script>alert(1)</script>",
			wantErr: ErrNameInjection,
		},
		{
			name:    "ampersand injection",
			input:   "User & Co",
			wantErr: ErrNameInjection,
		},
		{
			name:    "shell command substitution dollar sign",
			input:   "$(rm -rf)",
			wantErr: ErrNameInjection,
		},
		{
			name:    "pipe character",
			input:   "foo|bar",
			wantErr: ErrNameInjection,
		},
		{
			name:    "semicolon",
			input:   "foo;bar",
			wantErr: ErrNameInjection,
		},
		{
			name:    "opening brace",
			input:   "foo{bar}",
			wantErr: ErrNameInjection,
		},
		{
			name:    "greater-than sign",
			input:   "foo>bar",
			wantErr: ErrNameInjection,
		},
		{
			name:    "less-than sign",
			input:   "foo<bar",
			wantErr: ErrNameInjection,
		},
		// Happy path cases — must return nil.
		{
			name:    "plain ASCII name",
			input:   "Alice",
			wantErr: nil,
		},
		{
			name:    "Korean multi-byte name",
			input:   koreanName,
			wantErr: nil,
		},
		{
			name:    "name with hyphen and underscore",
			input:   "User-1_2",
			wantErr: nil,
		},
		{
			name:    "name with parentheses (not in blocklist)",
			input:   "Alice (Admin)",
			wantErr: nil,
		},
		{
			name:    "exactly 500 bytes",
			input:   strings.Repeat("a", 500),
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidatePersonaName(tc.input)
			if tc.wantErr == nil {
				if got != nil {
					t.Errorf("ValidatePersonaName(%q) = %v; want nil", tc.input, got)
				}
				return
			}
			if !errors.Is(got, tc.wantErr) {
				t.Errorf("ValidatePersonaName(%q) = %v; want errors.Is(..., %v)", tc.input, got, tc.wantErr)
			}
		})
	}
}

// TestValidateProviderAPIKey covers REQ-OB-015 and AC-OB-018.
// SPEC §6.8 #5 (InvalidAPIKey)
func TestValidateProviderAPIKey(t *testing.T) {
	// A valid Anthropic key: prefix "sk-ant-" + exactly 20 URL-safe chars.
	validAnthropic := "sk-ant-abcdefghij1234567890"

	// A valid OpenAI key: prefix "sk-" + 20 URL-safe chars.
	validOpenAI := "sk-abc123def456ghi789jkl"

	// A valid Google key: "AIza" + exactly 35 URL-safe chars.
	validGoogle := "AIza" + strings.Repeat("A", 35)

	// A valid DeepSeek key: same shape as OpenAI.
	validDeepSeek := "sk-abc123def456ghi789jkl"

	cases := []struct {
		name     string
		provider string
		key      string
		wantErr  error
	}{
		// Anthropic
		{
			name:     "anthropic valid key",
			provider: "anthropic",
			key:      validAnthropic,
			wantErr:  nil,
		},
		{
			name:     "anthropic invalid key (wrong prefix)",
			provider: "anthropic",
			key:      "sk-INVALID-123",
			wantErr:  ErrInvalidAPIKeyFormat,
		},
		{
			name:     "anthropic empty key",
			provider: "anthropic",
			key:      "",
			wantErr:  ErrEmptyAPIKey,
		},
		// OpenAI
		{
			name:     "openai valid key",
			provider: "openai",
			key:      validOpenAI,
			wantErr:  nil,
		},
		{
			name:     "openai key with google prefix",
			provider: "openai",
			key:      "AIzaXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
			wantErr:  ErrInvalidAPIKeyFormat,
		},
		{
			name:     "openai empty key",
			provider: "openai",
			key:      "",
			wantErr:  ErrEmptyAPIKey,
		},
		// Google
		{
			name:     "google valid key",
			provider: "google",
			key:      validGoogle,
			wantErr:  nil,
		},
		{
			name:     "google key too short",
			provider: "google",
			key:      "AIzashort",
			wantErr:  ErrInvalidAPIKeyFormat,
		},
		{
			name:     "google empty key",
			provider: "google",
			key:      "",
			wantErr:  ErrEmptyAPIKey,
		},
		// DeepSeek
		{
			name:     "deepseek valid key",
			provider: "deepseek",
			key:      validDeepSeek,
			wantErr:  nil,
		},
		{
			name:     "deepseek invalid key",
			provider: "deepseek",
			key:      "invalid-key",
			wantErr:  ErrInvalidAPIKeyFormat,
		},
		// Ollama — no key required
		{
			name:     "ollama empty key allowed",
			provider: "ollama",
			key:      "",
			wantErr:  nil,
		},
		{
			name:     "ollama non-empty key also allowed",
			provider: "ollama",
			key:      "some-value",
			wantErr:  nil,
		},
		// Unset — no key required
		{
			name:     "unset empty key allowed",
			provider: "unset",
			key:      "",
			wantErr:  nil,
		},
		// Custom — any non-empty value accepted
		{
			name:     "custom non-empty key accepted",
			provider: "custom",
			key:      "anything-goes-here",
			wantErr:  nil,
		},
		{
			name:     "custom empty key rejected",
			provider: "custom",
			key:      "",
			wantErr:  ErrEmptyAPIKey,
		},
		// Unknown provider
		{
			name:     "unknown provider xai",
			provider: "xai",
			key:      "anything",
			wantErr:  ErrUnknownProvider,
		},
		{
			name:     "empty provider string",
			provider: "",
			key:      "anything",
			wantErr:  ErrUnknownProvider,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateProviderAPIKey(tc.provider, tc.key)
			if tc.wantErr == nil {
				if got != nil {
					t.Errorf("ValidateProviderAPIKey(%q, %q) = %v; want nil", tc.provider, tc.key, got)
				}
				return
			}
			if !errors.Is(got, tc.wantErr) {
				t.Errorf("ValidateProviderAPIKey(%q, %q) = %v; want errors.Is(..., %v)",
					tc.provider, tc.key, got, tc.wantErr)
			}
		})
	}
}

// TestValidateGDPRConsent covers REQ-OB-008, AC-OB-008, and AC-OB-014.
// SPEC §6.8 #6 (GDPR consent)
func TestValidateGDPRConsent(t *testing.T) {
	cases := []struct {
		name    string
		consent ConsentFlags
		locale  LocaleChoice
		wantErr error
	}{
		{
			name:    "EU locale GDPR flag with nil consent",
			consent: ConsentFlags{GDPRExplicitConsent: nil},
			locale:  LocaleChoice{LegalFlags: []string{"GDPR"}},
			wantErr: ErrGDPRConsentRequired,
		},
		{
			name:    "EU locale GDPR flag with false consent",
			consent: ConsentFlags{GDPRExplicitConsent: ptrBool(false)},
			locale:  LocaleChoice{LegalFlags: []string{"GDPR"}},
			wantErr: ErrGDPRConsentRequired,
		},
		{
			name:    "EU locale GDPR flag with true consent",
			consent: ConsentFlags{GDPRExplicitConsent: ptrBool(true)},
			locale:  LocaleChoice{LegalFlags: []string{"GDPR"}},
			wantErr: nil,
		},
		{
			name:    "case-insensitive match: lowercase gdpr with nil consent",
			consent: ConsentFlags{GDPRExplicitConsent: nil},
			locale:  LocaleChoice{LegalFlags: []string{"gdpr"}},
			wantErr: ErrGDPRConsentRequired,
		},
		{
			name:    "case-insensitive match: mixed case Gdpr with true consent",
			consent: ConsentFlags{GDPRExplicitConsent: ptrBool(true)},
			locale:  LocaleChoice{LegalFlags: []string{"Gdpr"}},
			wantErr: nil,
		},
		{
			name:    "UK GDPR flag with nil consent",
			consent: ConsentFlags{GDPRExplicitConsent: nil},
			locale:  LocaleChoice{LegalFlags: []string{"UK_GDPR"}},
			// UK_GDPR does not equal "GDPR" (case-insensitive) — not in scope for this check.
			wantErr: nil,
		},
		{
			name:    "non-EU CCPA locale with nil consent",
			consent: ConsentFlags{GDPRExplicitConsent: nil},
			locale:  LocaleChoice{LegalFlags: []string{"CCPA"}},
			wantErr: nil,
		},
		{
			name:    "empty legal flags with nil consent",
			consent: ConsentFlags{GDPRExplicitConsent: nil},
			locale:  LocaleChoice{LegalFlags: []string{}},
			wantErr: nil,
		},
		{
			name:    "nil legal flags slice with nil consent",
			consent: ConsentFlags{GDPRExplicitConsent: nil},
			locale:  LocaleChoice{LegalFlags: nil},
			wantErr: nil,
		},
		{
			name:    "multiple flags including GDPR with true consent",
			consent: ConsentFlags{GDPRExplicitConsent: ptrBool(true)},
			locale:  LocaleChoice{LegalFlags: []string{"CCPA", "GDPR", "PIPL"}},
			wantErr: nil,
		},
		{
			name:    "multiple flags including GDPR with nil consent",
			consent: ConsentFlags{GDPRExplicitConsent: nil},
			locale:  LocaleChoice{LegalFlags: []string{"CCPA", "GDPR", "PIPL"}},
			wantErr: ErrGDPRConsentRequired,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateGDPRConsent(tc.consent, tc.locale)
			if tc.wantErr == nil {
				if got != nil {
					t.Errorf("ValidateGDPRConsent() = %v; want nil", got)
				}
				return
			}
			if !errors.Is(got, tc.wantErr) {
				t.Errorf("ValidateGDPRConsent() = %v; want errors.Is(..., %v)", got, tc.wantErr)
			}
		})
	}
}

// TestValidateHonorificLevel covers the HonorificLevel enum integrity check.
func TestValidateHonorificLevel(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		// Happy path: all valid enum values.
		{
			name:    "formal is valid",
			input:   "formal",
			wantErr: nil,
		},
		{
			name:    "casual is valid",
			input:   "casual",
			wantErr: nil,
		},
		{
			name:    "intimate is valid",
			input:   "intimate",
			wantErr: nil,
		},
		// Error path: case-sensitive enum — uppercase variants are invalid.
		{
			name:    "FORMAL uppercase is invalid",
			input:   "FORMAL",
			wantErr: ErrInvalidHonorificLevel,
		},
		{
			name:    "Casual title-case is invalid",
			input:   "Casual",
			wantErr: ErrInvalidHonorificLevel,
		},
		// Error path: unrecognized values.
		{
			name:    "empty string is invalid",
			input:   "",
			wantErr: ErrInvalidHonorificLevel,
		},
		{
			name:    "polite is not a valid enum value",
			input:   "polite",
			wantErr: ErrInvalidHonorificLevel,
		},
		{
			name:    "formal with trailing space is invalid",
			input:   "formal ",
			wantErr: ErrInvalidHonorificLevel,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateHonorificLevel(tc.input)
			if tc.wantErr == nil {
				if got != nil {
					t.Errorf("ValidateHonorificLevel(%q) = %v; want nil", tc.input, got)
				}
				return
			}
			if !errors.Is(got, tc.wantErr) {
				t.Errorf("ValidateHonorificLevel(%q) = %v; want errors.Is(..., %v)", tc.input, got, tc.wantErr)
			}
		})
	}
}

// TestValidateNoSensitiveFields covers REQ-OB-016 and AC-OB-019.
// SPEC §6.8 #12 (FieldWhitelist)
func TestValidateNoSensitiveFields(t *testing.T) {
	cases := []struct {
		name      string
		fieldName string
		wantErr   error
	}{
		// Empty input — no check needed.
		{
			name:      "empty field name returns nil",
			fieldName: "",
			wantErr:   nil,
		},
		// Whitelist members — all must return nil.
		{
			name:      "name is in whitelist",
			fieldName: "name",
			wantErr:   nil,
		},
		{
			name:      "honorific_level is in whitelist",
			fieldName: "honorific_level",
			wantErr:   nil,
		},
		{
			name:      "soul_markdown is in whitelist",
			fieldName: "soul_markdown",
			wantErr:   nil,
		},
		{
			name:      "gdpr_explicit_consent is in whitelist",
			fieldName: "gdpr_explicit_consent",
			wantErr:   nil,
		},
		{
			name:      "session_id is in whitelist",
			fieldName: "session_id",
			wantErr:   nil,
		},
		{
			name:      "api_key_stored is in whitelist",
			fieldName: "api_key_stored",
			wantErr:   nil,
		},
		{
			name:      "detected_tools is in whitelist",
			fieldName: "detected_tools",
			wantErr:   nil,
		},
		// Blocklist members — all must return ErrSensitiveFieldDetected.
		{
			name:      "ssn is prohibited",
			fieldName: "ssn",
			wantErr:   ErrSensitiveFieldDetected,
		},
		{
			name:      "phone_number is prohibited",
			fieldName: "phone_number",
			wantErr:   ErrSensitiveFieldDetected,
		},
		{
			name:      "email is prohibited",
			fieldName: "email",
			wantErr:   ErrSensitiveFieldDetected,
		},
		{
			name:      "physical_address is prohibited",
			fieldName: "physical_address",
			wantErr:   ErrSensitiveFieldDetected,
		},
		{
			name:      "biometric is prohibited",
			fieldName: "biometric",
			wantErr:   ErrSensitiveFieldDetected,
		},
		{
			name:      "medical is prohibited",
			fieldName: "medical",
			wantErr:   ErrSensitiveFieldDetected,
		},
		{
			name:      "government_id is prohibited",
			fieldName: "government_id",
			wantErr:   ErrSensitiveFieldDetected,
		},
		// Unknown fields — neither whitelist nor blocklist → ErrUnknownField.
		{
			name:      "favorite_color is unknown",
			fieldName: "favorite_color",
			wantErr:   ErrUnknownField,
		},
		{
			name:      "random_thing is unknown",
			fieldName: "random_thing",
			wantErr:   ErrUnknownField,
		},
		{
			name:      "user_id is unknown (not in whitelist)",
			fieldName: "user_id",
			wantErr:   ErrUnknownField,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateNoSensitiveFields(tc.fieldName)
			if tc.wantErr == nil {
				if got != nil {
					t.Errorf("ValidateNoSensitiveFields(%q) = %v; want nil", tc.fieldName, got)
				}
				return
			}
			if !errors.Is(got, tc.wantErr) {
				t.Errorf("ValidateNoSensitiveFields(%q) = %v; want errors.Is(..., %v)",
					tc.fieldName, got, tc.wantErr)
			}
		})
	}
}
