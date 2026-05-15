// Package onboarding — validators.go contains pure validation helpers for the 7-step
// onboarding state machine. All functions are stateless: no file I/O, no OS keyring,
// no logging, no network calls. Side effects (security event logging, keyring writes)
// are performed by higher-level phases (Phase 1C).
// SPEC: SPEC-MINK-ONBOARDING-001 §6.8
// REQ: REQ-OB-008, REQ-OB-015, REQ-OB-016, REQ-OB-017
package onboarding

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// personaNameInjectionPattern matches characters that could enable HTML or shell injection.
// Allowed set rejection: < > & { } ; | $
// REQ-OB-017: "shell/HTML injection patterns (per regex [<>&{};|$])"
// Compiled once at init to avoid repeated compilation overhead.
var personaNameInjectionPattern = regexp.MustCompile(`[<>&{};|$]`)

// providerKeyPatterns maps provider identifiers to their required API key regex.
// Patterns are derived from AC-OB-018 and REQ-OB-015.
//
// "anthropic" → prefix "sk-ant-" followed by >=20 URL-safe chars
// "openai"    → prefix "sk-" followed by >=20 URL-safe chars (covers sk-proj- variants)
// "google"    → prefix "AIza" followed by exactly 35 URL-safe chars (total 39 chars)
// "deepseek"  → prefix "sk-" followed by >=20 URL-safe chars (same shape as openai)
//
// Providers whose keys are optional ("ollama", "unset") or accept any non-empty value
// ("custom") are handled inline in ValidateProviderAPIKey and are not in this map.
var providerKeyPatterns = map[string]*regexp.Regexp{
	"anthropic": regexp.MustCompile(`^sk-ant-[A-Za-z0-9_-]{20,}$`),
	"openai":    regexp.MustCompile(`^sk-[A-Za-z0-9_-]{20,}$`),
	"google":    regexp.MustCompile(`^AIza[A-Za-z0-9_-]{35}$`),
	"deepseek":  regexp.MustCompile(`^sk-[A-Za-z0-9_-]{20,}$`),
}

// providerKeyPrefixHint maps each provider to a human-readable expected prefix,
// used in ErrInvalidAPIKeyFormat wrapped messages for diagnostic clarity.
var providerKeyPrefixHint = map[string]string{
	"anthropic": "sk-ant-",
	"openai":    "sk-",
	"google":    "AIza",
	"deepseek":  "sk-",
}

// sensitiveFieldBlocklist enumerates field names that are explicitly prohibited
// by REQ-OB-016 (PII / government ID / biometric categories).
var sensitiveFieldBlocklist = map[string]bool{
	"ssn":              true,
	"biometric":        true,
	"medical":          true,
	"government_id":    true,
	"phone_number":     true,
	"physical_address": true,
	"email":            true,
}

// fieldWhitelist is the complete set of allowed onboarding field names per REQ-OB-016.
// Field names are derived from the OnboardingData struct layout and SPEC §6.2.
var fieldWhitelist = map[string]bool{
	// PersonaProfile (Step 4)
	"name":            true,
	"honorific_level": true,
	"pronouns":        true,
	"soul_markdown":   true,
	// LocaleChoice (Step 1)
	"locale_choice": true,
	"country":       true,
	"language":      true,
	"timezone":      true,
	"legal_flags":   true,
	// ProviderChoice (Step 5)
	"provider":        true,
	"auth_method":     true,
	"api_key_stored":  true,
	"custom_endpoint": true,
	"preferred_model": true,
	// MessengerChannel (Step 6)
	"messenger_type": true,
	"bot_token_key":  true,
	// ConsentFlags (Step 7)
	"conversation_storage_local": true,
	"lora_training_allowed":      true,
	"telemetry_enabled":          true,
	"crash_reporting_enabled":    true,
	"gdpr_explicit_consent":      true,
	// ModelSetup (Step 2)
	"ollama_installed": true,
	"detected_model":   true,
	"selected_model":   true,
	"model_size_bytes": true,
	"ram_bytes":        true,
	// CLITool (Step 3)
	"detected_tools": true,
	"tool_name":      true,
	"tool_version":   true,
	"tool_path":      true,
	// OnboardingFlow top-level fields
	"session_id":   true,
	"current_step": true,
	"data":         true,
	"started_at":   true,
	"completed_at": true,
}

// ValidatePersonaName checks the Step 4 Persona name field per REQ-OB-017.
//
// Rejection conditions (return non-nil error):
//   - empty or whitespace-only → ErrNameEmpty (AC-OB-005)
//   - length > 500 bytes (UTF-8) → ErrNameTooLong (REQ-OB-017)
//     Note: the spec says "500+ characters" but this implementation uses byte length
//     (len(name) > 500) for defence-in-depth against multi-byte padding exploits.
//     Valid Unicode names up to 500 bytes are accepted; names that are shorter in
//     rune count but longer in bytes are rejected.
//   - contains any of < > & { } ; | $ (shell/HTML injection chars) → ErrNameInjection (AC-OB-015)
//
// Returns nil for a valid name.
//
// Security event logging to ./.mink/security-events.log (AC-OB-015 Then clause) is the
// responsibility of the caller (Phase 1C / flow wiring); this function is pure.
//
// REQ: REQ-OB-017
// AC: AC-OB-005, AC-OB-015
//
// @MX:NOTE: [AUTO] Length check uses byte length, not rune count — see docstring.
func ValidatePersonaName(name string) error {
	if strings.TrimSpace(name) == "" {
		return ErrNameEmpty
	}

	if len(name) > 500 {
		return ErrNameTooLong
	}

	// utf8.ValidString is a precaution; the injection check is the security gate.
	if utf8.ValidString(name) && personaNameInjectionPattern.MatchString(name) {
		return ErrNameInjection
	}
	// Also check if the string is not valid UTF-8 — in that case we run the pattern
	// on whatever bytes are present to be safe.
	if !utf8.ValidString(name) && personaNameInjectionPattern.MatchString(name) {
		return ErrNameInjection
	}

	return nil
}

// ValidateProviderAPIKey checks the Step 5 Provider api_key field per REQ-OB-015.
//
// Provider-specific prefix regex table (matches AC-OB-018):
//   - "anthropic" → `^sk-ant-[A-Za-z0-9_-]{20,}$`
//   - "openai"    → `^sk-[A-Za-z0-9_-]{20,}$`  (also matches sk-proj- variants)
//   - "google"    → `^AIza[A-Za-z0-9_-]{35}$`
//   - "deepseek"  → `^sk-[A-Za-z0-9_-]{20,}$`
//   - "ollama"    → empty key allowed (returns nil for "")
//   - "unset"     → empty key allowed (returns nil for "")
//   - "custom"    → any non-empty string accepted (returns nil for any non-empty)
//
// Unknown provider → ErrUnknownProvider.
// Empty key for provider that requires one → ErrEmptyAPIKey.
// Pattern mismatch → ErrInvalidAPIKeyFormat wrapped with provider prefix hint.
//
// REQ: REQ-OB-015
// AC: AC-OB-018
//
// @MX:ANCHOR: [AUTO] Central API key validation — fan_in >= 3 (SubmitStep wiring, CLI TUI,
// Web UI handler). Any change to the regex table affects all callers.
// @MX:REASON: Provider key patterns are a security invariant; changes require coordinated
// update of all callers and corresponding test coverage.
func ValidateProviderAPIKey(provider string, key string) error {
	switch provider {
	case "ollama", "unset":
		// These providers do not require an API key; any value (including empty) is valid.
		return nil

	case "custom":
		// Custom endpoints accept any non-empty key; the format is user-defined.
		if key == "" {
			return ErrEmptyAPIKey
		}
		return nil

	case "anthropic", "openai", "google", "deepseek":
		if key == "" {
			return ErrEmptyAPIKey
		}
		pattern, ok := providerKeyPatterns[provider]
		if !ok {
			// Defensive: should not happen given the outer switch arms.
			return ErrUnknownProvider
		}
		if !pattern.MatchString(key) {
			hint := providerKeyPrefixHint[provider]
			return fmt.Errorf("invalid API key format for %s (expected prefix %q): %w",
				provider, hint, ErrInvalidAPIKeyFormat)
		}
		return nil

	default:
		return ErrUnknownProvider
	}
}

// ValidateGDPRConsent enforces REQ-OB-008 / AC-OB-008 / AC-OB-014.
//
// Logic:
//   - If locale.LegalFlags contains "GDPR" (case-insensitive element comparison),
//     then consent.GDPRExplicitConsent MUST be non-nil AND *true;
//     otherwise returns ErrGDPRConsentRequired.
//   - If LegalFlags has no "GDPR" entry, GDPRExplicitConsent is irrelevant; returns nil.
//
// Locale legal flags membership uses strings.EqualFold per element (case-insensitive).
//
// Note: locale-aware error message translation (AC-OB-014 French message) is deferred
// to Phase 2 when I18N-001 lands.
// @MX:NOTE: [AUTO] I18N-001 dep — French locale error message deferred to Phase 2.
//
// REQ: REQ-OB-008
// AC: AC-OB-008, AC-OB-014
func ValidateGDPRConsent(consent ConsentFlags, locale LocaleChoice) error {
	gdprRequired := false
	for _, flag := range locale.LegalFlags {
		if strings.EqualFold(flag, "GDPR") {
			gdprRequired = true
			break
		}
	}

	if !gdprRequired {
		return nil
	}

	// GDPR jurisdiction: consent must be explicitly and affirmatively given.
	if consent.GDPRExplicitConsent == nil || !*consent.GDPRExplicitConsent {
		return ErrGDPRConsentRequired
	}

	return nil
}

// ValidateHonorificLevel enforces the HonorificLevel string enum.
//
// Allowed values (case-sensitive): "formal" | "casual" | "intimate".
// Empty string returns ErrInvalidHonorificLevel (caller decides whether to apply a default).
// Any other value → ErrInvalidHonorificLevel wrapped with the offending value.
//
// REQ: implicit (enum integrity for Step 4 PersonaProfile.HonorificLevel)
func ValidateHonorificLevel(level string) error {
	switch HonorificLevel(level) {
	case HonorificFormal, HonorificCasual, HonorificIntimate:
		return nil
	default:
		if level == "" {
			return ErrInvalidHonorificLevel
		}
		return fmt.Errorf("%w: got %q", ErrInvalidHonorificLevel, level)
	}
}

// ValidateNoSensitiveFields performs the REQ-OB-016 / AC-OB-019 whitelist check for a
// single field name. The static audit script (test/audit_no_sensitive_fields.go) calls
// this function once per schema field to verify no prohibited fields are present.
//
// Allowed field names (case-sensitive whitelist): see fieldWhitelist map above.
//
// Prohibited (REQ-OB-016): ssn, biometric, medical, government_id, phone_number,
// physical_address, email — these return ErrSensitiveFieldDetected (wrapped with field name).
//
// Unknown fields (not in whitelist and not in blocklist) → ErrUnknownField (wrapped with name).
//
// Empty input → returns nil (no field to check).
//
// REQ: REQ-OB-016
// AC: AC-OB-019
func ValidateNoSensitiveFields(fieldName string) error {
	if fieldName == "" {
		return nil
	}

	// Check blocklist first: prohibited fields are an explicit security boundary.
	if sensitiveFieldBlocklist[fieldName] {
		return fmt.Errorf("%w: %q", ErrSensitiveFieldDetected, fieldName)
	}

	// Check whitelist: fields not explicitly permitted are also rejected.
	if !fieldWhitelist[fieldName] {
		return fmt.Errorf("%w: %q", ErrUnknownField, fieldName)
	}

	return nil
}

// Sentinel errors for the validator functions.
// Callers use errors.Is to distinguish error categories.
// Wrapped variants (fmt.Errorf("%w: ...", ErrXxx)) are compatible with errors.Is.
var (
	// ErrNameEmpty is returned when the persona name is empty or whitespace-only.
	// AC-OB-005
	ErrNameEmpty = errors.New("persona name is required")

	// ErrNameTooLong is returned when the persona name exceeds 500 bytes.
	// REQ-OB-017
	ErrNameTooLong = errors.New("persona name exceeds 500 bytes")

	// ErrNameInjection is returned when the persona name contains shell/HTML injection chars.
	// AC-OB-015
	ErrNameInjection = errors.New("persona name contains prohibited characters")

	// ErrUnknownProvider is returned when the provider string is not in the known set.
	ErrUnknownProvider = errors.New("unknown provider")

	// ErrEmptyAPIKey is returned when a provider that requires a key receives an empty string.
	// REQ-OB-015
	ErrEmptyAPIKey = errors.New("API key is required for the selected provider")

	// ErrInvalidAPIKeyFormat is returned when the key does not match the provider's pattern.
	// AC-OB-018
	ErrInvalidAPIKeyFormat = errors.New("invalid API key format")

	// ErrGDPRConsentRequired is returned when GDPR jurisdiction requires explicit consent
	// but GDPRExplicitConsent is nil or false.
	// REQ-OB-008, AC-OB-008, AC-OB-014
	ErrGDPRConsentRequired = errors.New("explicit GDPR consent is required")

	// ErrInvalidHonorificLevel is returned for any value not in the HonorificLevel enum.
	ErrInvalidHonorificLevel = errors.New("invalid honorific level (must be formal, casual, or intimate)")

	// ErrSensitiveFieldDetected is returned when a blocklisted field name is encountered.
	// REQ-OB-016, AC-OB-019
	ErrSensitiveFieldDetected = errors.New("sensitive field detected (whitelist violation)")

	// ErrUnknownField is returned when a field name is neither in the whitelist nor blocklist.
	// REQ-OB-016
	ErrUnknownField = errors.New("unknown field name")
)
