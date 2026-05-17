// Package credential — sentinel error definitions.
//
// All errors are simple sentinel values created with errors.New.  Callers
// should use errors.Is() for comparison so that wrapping is preserved.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (SD-1, UB-8)
package credential

import "errors"

// ErrKeyringUnavailable is returned by the keyring backend when the OS
// keyring service is not reachable (e.g. headless Linux without D-Bus,
// locked macOS Keychain in SSH session, or Windows Wincred timeout).
//
// Callers should treat this as a signal to either surface a helpful message
// or transparently fall back to the file backend when the dispatcher is
// configured for auto-fallback mode (SD-1).
var ErrKeyringUnavailable = errors.New("keyring unavailable")

// ErrNotFound is returned when a requested provider credential does not exist
// in the active backend.  Callers such as LLM-ROUTING-V2 use this sentinel to
// silently skip the provider (SD-3).
var ErrNotFound = errors.New("credential not found")

// ErrSchemaViolation is returned by Credential.Validate() when the payload
// does not conform to the declared schema for its Kind (UB-6).
var ErrSchemaViolation = errors.New("credential schema violation")

// ErrReAuthRequired is returned during a Codex OAuth refresh when the
// refresh_token is revoked or has expired past its 8-day idle window
// (ED-5). The provider identifier is embedded in the wrapping error so
// callers can route the user to the correct re-authentication command.
var ErrReAuthRequired = errors.New("re-authentication required")

// IsKeyringUnavailable reports whether err is or wraps ErrKeyringUnavailable.
func IsKeyringUnavailable(err error) bool {
	return errors.Is(err, ErrKeyringUnavailable)
}

// IsNotFound reports whether err is or wraps ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsSchemaViolation reports whether err is or wraps ErrSchemaViolation.
func IsSchemaViolation(err error) bool {
	return errors.Is(err, ErrSchemaViolation)
}

// IsReAuthRequired reports whether err is or wraps ErrReAuthRequired.
func IsReAuthRequired(err error) bool {
	return errors.Is(err, ErrReAuthRequired)
}
