package credential_test

import (
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// TestAPIKeyKind verifies that the Kind() method returns KindAPIKey.
func TestAPIKeyKind(t *testing.T) {
	t.Parallel()

	a := credential.APIKey{Value: "sk-test"}
	if a.Kind() != credential.KindAPIKey {
		t.Errorf("Kind() = %q; want %q", a.Kind(), credential.KindAPIKey)
	}
}

// TestAPIKeyValidateHappyPath verifies that a properly populated APIKey
// passes validation.
func TestAPIKeyValidateHappyPath(t *testing.T) {
	t.Parallel()

	a := credential.APIKey{Value: "sk-ant-api03-test"}
	if err := a.Validate(); err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
	}
}

// TestAPIKeyValidateMissingValue verifies that an empty Value triggers
// ErrSchemaViolation.
func TestAPIKeyValidateMissingValue(t *testing.T) {
	t.Parallel()

	a := credential.APIKey{}
	err := a.Validate()
	if err == nil {
		t.Fatal("Validate() expected error for empty Value, got nil")
	}
	if !errors.Is(err, credential.ErrSchemaViolation) {
		t.Errorf("Validate() error %v does not wrap ErrSchemaViolation", err)
	}
}

// TestAPIKeyMaskedString verifies that MaskedString() never leaks plaintext.
func TestAPIKeyMaskedString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "short", value: "abc"},
		{name: "long", value: "sk-ant-api03-XXXXXXXXXXXXXXXXXXXXXXXXXXXX"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			a := credential.APIKey{Value: tc.value}
			masked := a.MaskedString()

			if masked == tc.value && len(tc.value) >= 5 {
				t.Errorf("MaskedString() returned full plaintext for value of length %d", len(tc.value))
			}
		})
	}
}
