// Package commands — auth.store config key validation.
//
// This file extends the generic "mink config set" command with validation
// specific to the "auth.store" key (SPEC-MINK-AUTH-CREDENTIAL-001 T-008).
//
// Valid values: keyring | file | keyring,file
// Reserved (OP placeholder, not implemented): hsm | op-cli
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (SD-1, SD-2, OP-1, OP-2,
//
//	AC-CR-021, AC-CR-029, AC-CR-030, T-008)
package commands

import (
	"errors"
	"fmt"
	"slices"
)

// authStoreKey is the config key managed by this validation layer.
const authStoreKey = "auth.store"

// validAuthStoreValues lists the three accepted auth.store values.
var validAuthStoreValues = []string{"keyring", "file", "keyring,file"}

// opPlaceholderValues lists the OP placeholder values that are recognised but
// explicitly rejected with a NotImplemented message (OP-1, OP-2,
// AC-CR-029, AC-CR-030).
var opPlaceholderValues = []string{"hsm", "op-cli"}

// ErrAuthStoreNotImplemented is returned when the user requests an OP
// placeholder value (hsm or op-cli) that is reserved but not yet implemented.
var ErrAuthStoreNotImplemented = errors.New("auth.store: not implemented")

// validateAuthStoreValue checks whether value is acceptable for the auth.store
// key.
//
// Returns:
//   - nil                              — value is valid
//   - ErrAuthStoreNotImplemented (wrapped) — value is an OP placeholder
//   - an error with the allowed-values list — value is completely unknown
func validateAuthStoreValue(value string) error {
	if slices.Contains(validAuthStoreValues, value) {
		return nil
	}

	if slices.Contains(opPlaceholderValues, value) {
		return fmt.Errorf("%w: auth.store: %q reserved for future SPEC (OP-1/OP-2), not implemented",
			ErrAuthStoreNotImplemented, value)
	}

	return fmt.Errorf("auth.store: %q is not a valid value; accepted values: keyring, file, keyring,file",
		value)
}

// validateConfigSet applies key-specific validation rules before a config set
// operation is persisted.  Returns an error when the key+value pair fails
// validation; returns nil when it is valid or when no special rule applies to
// the key.
func validateConfigSet(key, value string) error {
	if key == authStoreKey {
		return validateAuthStoreValue(value)
	}
	// No special validation for other keys.
	return nil
}
