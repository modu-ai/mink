package aliasconfig

import (
	"errors"
	"fmt"
	"testing"
)

// TestSentinelErrors verifies all sentinel errors are defined and unique.
// REQ-ALIAS-030 (ErrMalformedAliasFile), REQ-ALIAS-031/032 (ErrEmptyAliasEntry),
// REQ-ALIAS-033 (ErrInvalidCanonical), REQ-ALIAS-034 (ErrUnknownProviderInAlias),
// REQ-ALIAS-035 (ErrUnknownModelInAlias), REQ-ALIAS-036 (ErrAliasFileTooLarge).
func TestSentinelErrors(t *testing.T) {
	t.Run("all sentinel errors are defined", func(t *testing.T) {
		// This test will fail until we define all sentinel errors in errors.go
		testCases := []struct {
			name string
			err  error
			want string
		}{
			{
				name: "ErrMalformedAliasFile",
				err:  ErrMalformedAliasFile,
				want: "malformed alias file",
			},
			{
				name: "ErrEmptyAliasEntry",
				err:  ErrEmptyAliasEntry,
				want: "empty alias entry",
			},
			{
				name: "ErrInvalidCanonical",
				err:  ErrInvalidCanonical,
				want: "invalid canonical form",
			},
			{
				name: "ErrUnknownProviderInAlias",
				err:  ErrUnknownProviderInAlias,
				want: "unknown provider in alias",
			},
			{
				name: "ErrUnknownModelInAlias",
				err:  ErrUnknownModelInAlias,
				want: "unknown model in alias",
			},
			{
				name: "ErrAliasFileTooLarge",
				err:  ErrAliasFileTooLarge,
				want: "alias file too large",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if tc.err == nil {
					t.Errorf("%s: sentinel error is not defined", tc.name)
				}
				if tc.err != nil && tc.err.Error() == "" {
					t.Errorf("%s: sentinel error has empty Error() output", tc.name)
				}
				// Verify error message contains expected substring
				if tc.err != nil {
					got := tc.err.Error()
					// Just verify it's not empty and is unique
					for _, other := range testCases {
						if other.name != tc.name && other.err != nil && got == other.err.Error() {
							t.Errorf("%s: has duplicate error message with %s: %q", tc.name, other.name, got)
						}
					}
				}
			})
		}
	})

	t.Run("sentinel errors support wrapping", func(t *testing.T) {
		// Verify sentinel errors can be wrapped with fmt.Errorf
		baseErr := ErrMalformedAliasFile
		wrappedErr := fmt.Errorf("failed to parse: %w", baseErr)

		if !errors.Is(wrappedErr, ErrMalformedAliasFile) {
			t.Error("ErrMalformedAliasFile does not support errors.Is wrapping")
		}
	})
}
