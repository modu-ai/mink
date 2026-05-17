package credential_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// TestSentinelIdentity verifies that each sentinel error has a stable identity
// and that errors.Is() works when sentinels are wrapped.
//
// AC-CR-024 (indirect): masking and sentinel tests form the base layer for
// the UN-1 requirement.
func TestSentinelIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sentinel  error
		checkFunc func(error) bool
	}{
		{
			name:      "ErrKeyringUnavailable",
			sentinel:  credential.ErrKeyringUnavailable,
			checkFunc: credential.IsKeyringUnavailable,
		},
		{
			name:      "ErrNotFound",
			sentinel:  credential.ErrNotFound,
			checkFunc: credential.IsNotFound,
		},
		{
			name:      "ErrSchemaViolation",
			sentinel:  credential.ErrSchemaViolation,
			checkFunc: credential.IsSchemaViolation,
		},
		{
			name:      "ErrReAuthRequired",
			sentinel:  credential.ErrReAuthRequired,
			checkFunc: credential.IsReAuthRequired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Direct equality.
			if !errors.Is(tc.sentinel, tc.sentinel) {
				t.Errorf("%s: errors.Is(sentinel, sentinel) = false", tc.name)
			}

			// Wrapped equality — the Is* helper must still detect it.
			wrapped := fmt.Errorf("outer: %w", tc.sentinel)
			if !tc.checkFunc(wrapped) {
				t.Errorf("%s: Is* helper returned false for wrapped sentinel", tc.name)
			}

			// Cross-sentinel negative test — ErrNotFound must not match
			// ErrKeyringUnavailable etc.
			if tc.sentinel == credential.ErrNotFound {
				if credential.IsKeyringUnavailable(tc.sentinel) {
					t.Errorf("ErrNotFound incorrectly matched by IsKeyringUnavailable")
				}
			}
		})
	}
}
