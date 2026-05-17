// Package credential — credential masking helper for safe logging.
//
// All log output involving credential values MUST pass through MaskedString
// to prevent plaintext leakage (UN-1).
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UN-1)
package credential

// MaskedString returns a safe representation of s for use in logs, health
// responses, and CLI output.  Plaintext is never returned.
//
// Rules:
//   - len(s) >= 5: "***" + last-4 characters (e.g. "sk-ant-1234567890" → "***7890")
//   - len(s) < 5:  "***" (token too short to reveal any suffix safely)
//   - s == "":     "***"
//
// @MX:ANCHOR: [AUTO] MaskedString is called from every credential type's
// MaskedString() method and from Health() in the keyring backend (fan_in >= 3).
// @MX:REASON: Masking contract is security-critical — any change here
// affects all credential types and logging paths simultaneously.
// @MX:SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UN-1, AC-CR-024)
func MaskedString(s string) string {
	runes := []rune(s)
	if len(runes) < 5 {
		return "***"
	}
	// Show only the last 4 runes; prefix with *** to indicate truncation.
	return "***" + string(runes[len(runes)-4:])
}
