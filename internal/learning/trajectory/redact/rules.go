package redact

import "regexp"

// BuiltinRules returns the 6 default PII redaction rules.
// Order follows spec.md §6.6 built-in precedence.
//
// Rules are applied in this order after any user-supplied rules:
//  1. email
//  2. openai_key
//  3. bearer_jwt
//  4. credit_card (Luhn-validated)
//  5. kr_phone
//  6. home_path
//
// @MX:ANCHOR: BuiltinRules is the canonical PII redaction surface for all Trajectory data.
// @MX:REASON: Called by NewChain and NewBuiltinChain; any consumer of the redact package
// relies on this list for privacy compliance. Adding, removing, or reordering rules here
// affects downstream LoRA training data quality and GDPR Art.25 compliance guarantees.
func BuiltinRules() []Rule {
	return []Rule{
		{
			Name:        "email",
			Pattern:     regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
			Replacement: "<REDACTED:email>",
		},
		{
			Name:        "openai_key",
			Pattern:     regexp.MustCompile(`sk-[A-Za-z0-9\-_]{20,}`),
			Replacement: "<REDACTED:api_key>",
		},
		{
			Name:        "bearer_jwt",
			Pattern:     regexp.MustCompile(`Bearer\s+ey[A-Za-z0-9\-_.]+`),
			Replacement: "Bearer <REDACTED:jwt>",
		},
		{
			Name:            "credit_card",
			Pattern:         regexp.MustCompile(`\b(?:\d[ \-]*?){13,16}\b`),
			Replacement:     "<REDACTED:cc>",
			AppliesToSystem: false,
		},
		{
			Name:        "kr_phone",
			Pattern:     regexp.MustCompile(`\b01[016789]-\d{3,4}-\d{4}\b`),
			Replacement: "<REDACTED:phone>",
		},
		{
			Name:        "home_path",
			Pattern:     regexp.MustCompile(`(/Users|/home)/[a-zA-Z][a-zA-Z0-9_\-]{1,30}`),
			Replacement: "$1/<REDACTED:user>",
		},
	}
}
