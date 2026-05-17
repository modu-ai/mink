// Package commands — tests for auth.store config key validation (T-008).
//
// Tests cover:
//   - Three valid auth.store values are accepted
//   - OP placeholder values (hsm, op-cli) are rejected with NotImplemented
//   - Arbitrary garbage values are rejected with a descriptive error
//   - Integration with the full "mink config set" cobra command
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (AC-CR-021, AC-CR-029, AC-CR-030, T-008)
package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestValidateAuthStoreValueValid verifies that the three accepted values pass.
func TestValidateAuthStoreValueValid(t *testing.T) {
	for _, v := range []string{"keyring", "file", "keyring,file"} {
		t.Run(v, func(t *testing.T) {
			if err := validateAuthStoreValue(v); err != nil {
				t.Errorf("validateAuthStoreValue(%q): unexpected error: %v", v, err)
			}
		})
	}
}

// TestValidateAuthStoreValueOPPlaceholderRejected verifies that hsm and op-cli
// are rejected with ErrAuthStoreNotImplemented (AC-CR-029, AC-CR-030).
func TestValidateAuthStoreValueOPPlaceholderRejected(t *testing.T) {
	for _, v := range []string{"hsm", "op-cli"} {
		t.Run(v, func(t *testing.T) {
			err := validateAuthStoreValue(v)
			if err == nil {
				t.Fatalf("validateAuthStoreValue(%q): expected error, got nil", v)
			}
			if !errors.Is(err, ErrAuthStoreNotImplemented) {
				t.Errorf("validateAuthStoreValue(%q): expected ErrAuthStoreNotImplemented, got %v", v, err)
			}
			// Error message must include "reserved for future SPEC (OP-1/OP-2)" guidance.
			if !strings.Contains(err.Error(), "OP-1/OP-2") {
				t.Errorf("error message should mention OP-1/OP-2, got: %s", err.Error())
			}
		})
	}
}

// TestValidateAuthStoreValueGarbageRejected verifies that arbitrary unknown
// values return a descriptive error (not ErrAuthStoreNotImplemented).
func TestValidateAuthStoreValueGarbageRejected(t *testing.T) {
	for _, v := range []string{"vault", "aws-kms", "", "KEYRING", "File"} {
		t.Run(v, func(t *testing.T) {
			err := validateAuthStoreValue(v)
			if err == nil {
				t.Fatalf("validateAuthStoreValue(%q): expected error, got nil", v)
			}
			if errors.Is(err, ErrAuthStoreNotImplemented) {
				t.Errorf("validateAuthStoreValue(%q): should be a generic invalid-value error, not NotImplemented", v)
			}
		})
	}
}

// TestValidateConfigSetPassthroughForOtherKeys verifies that validateConfigSet
// returns nil for keys it does not own special validation for.
func TestValidateConfigSetPassthroughForOtherKeys(t *testing.T) {
	if err := validateConfigSet("log.level", "debug"); err != nil {
		t.Errorf("validateConfigSet(log.level, debug): unexpected error: %v", err)
	}
	if err := validateConfigSet("some.random.key", "any-value"); err != nil {
		t.Errorf("validateConfigSet(some.random.key, any-value): unexpected error: %v", err)
	}
}

// TestConfigSetCommandAuthStoreValidValues verifies end-to-end that the cobra
// command accepts valid auth.store values and persists them.
func TestConfigSetCommandAuthStoreValidValues(t *testing.T) {
	for _, v := range []string{"keyring", "file", "keyring,file"} {
		t.Run(v, func(t *testing.T) {
			store := newMockConfigStore()
			cmd := NewConfigCommand(store)
			cmd.SetArgs([]string{"set", "auth.store", v})
			cmd.SetOut(bytes.NewBuffer(nil))
			cmd.SetErr(bytes.NewBuffer(nil))

			if err := cmd.Execute(); err != nil {
				t.Errorf("Execute: unexpected error for valid value %q: %v", v, err)
			}

			got, err := store.Get("auth.store")
			if err != nil {
				t.Fatalf("Get auth.store: %v", err)
			}
			if got != v {
				t.Errorf("auth.store = %q, want %q", got, v)
			}
		})
	}
}

// TestConfigSetCommandAuthStoreOPPlaceholderRejected verifies end-to-end that
// the cobra command rejects hsm and op-cli (AC-CR-029, AC-CR-030).
func TestConfigSetCommandAuthStoreOPPlaceholderRejected(t *testing.T) {
	for _, v := range []string{"hsm", "op-cli"} {
		t.Run(v, func(t *testing.T) {
			store := newMockConfigStore()
			cmd := NewConfigCommand(store)
			cmd.SetArgs([]string{"set", "auth.store", v})
			cmd.SetOut(bytes.NewBuffer(nil))
			cmd.SetErr(bytes.NewBuffer(nil))

			err := cmd.Execute()
			if err == nil {
				t.Fatalf("Execute: expected error for OP placeholder %q, got nil", v)
			}

			// Store must NOT have been called.
			if _, getErr := store.Get("auth.store"); getErr == nil {
				t.Errorf("auth.store should not have been persisted for rejected value %q", v)
			}
		})
	}
}

// TestConfigSetCommandAuthStoreInvalidRejected verifies that an arbitrary
// invalid value is rejected.
func TestConfigSetCommandAuthStoreInvalidRejected(t *testing.T) {
	store := newMockConfigStore()
	cmd := NewConfigCommand(store)
	cmd.SetArgs([]string{"set", "auth.store", "vault-enterprise"})
	cmd.SetOut(bytes.NewBuffer(nil))
	cmd.SetErr(bytes.NewBuffer(nil))

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute: expected error for invalid auth.store value, got nil")
	}
}
