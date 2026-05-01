// Package aliasconfig — codes.go tests (T-005).
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 AC-AMEND-004, AC-AMEND-011
package aliasconfig

import (
	"errors"
	"fmt"
	"testing"

	"github.com/modu-ai/goose/internal/command"
)

// TestErrorCodeOf_KnownSentinels verifies that ErrorCodeOf returns the stable
// identifier for each sentinel error in the table, including wrapped chains.
// AC-AMEND-004 — REQ-AMEND-004.
func TestErrorCodeOf_KnownSentinels(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode string
	}{
		{"ConfigNotFound direct", ErrConfigNotFound, "ALIAS-001"},
		{"ConfigNotFound wrapped", fmt.Errorf("%w: missing", ErrConfigNotFound), "ALIAS-001"},
		{"EmptyAliasEntry", command.ErrEmptyAliasEntry, "ALIAS-010"},
		{"InvalidCanonical", command.ErrInvalidCanonical, "ALIAS-011"},
		{"AliasFileTooLarge", command.ErrAliasFileTooLarge, "ALIAS-020"},
		{"MalformedAliasFile", command.ErrMalformedAliasFile, "ALIAS-021"},
		{"UnknownProviderInAlias", command.ErrUnknownProviderInAlias, "ALIAS-030"},
		{"UnknownModelInAlias", command.ErrUnknownModelInAlias, "ALIAS-031"},
		{"InvalidCanonical double-wrapped", fmt.Errorf("layer: %w", fmt.Errorf("%w: name", command.ErrInvalidCanonical)), "ALIAS-011"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ErrorCodeOf(tc.err); got != tc.wantCode {
				t.Fatalf("ErrorCodeOf = %q, want %q", got, tc.wantCode)
			}
		})
	}
}

// TestErrorCodeOf_NilReturnsEmpty checks the nil contract.
// AC-AMEND-004.
func TestErrorCodeOf_NilReturnsEmpty(t *testing.T) {
	if got := ErrorCodeOf(nil); got != "" {
		t.Fatalf("ErrorCodeOf(nil) = %q, want empty string", got)
	}
}

// TestErrorCodeOf_UnrecognizedReturnsEmpty checks the unknown-error fallback.
// AC-AMEND-004 (negative path).
func TestErrorCodeOf_UnrecognizedReturnsEmpty(t *testing.T) {
	custom := errors.New("some unrelated error")
	if got := ErrorCodeOf(custom); got != "" {
		t.Fatalf("ErrorCodeOf(custom) = %q, want empty", got)
	}
}

// TestCategorize_KnownSentinels verifies category mapping for each sentinel.
// AC-AMEND-011 — REQ-AMEND-011.
func TestCategorize_KnownSentinels(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want ErrorCategory
	}{
		{"ConfigNotFound", ErrConfigNotFound, CategoryLoad},
		{"EmptyAliasEntry", command.ErrEmptyAliasEntry, CategoryFormat},
		{"InvalidCanonical", command.ErrInvalidCanonical, CategoryFormat},
		{"AliasFileTooLarge", command.ErrAliasFileTooLarge, CategorySize},
		{"MalformedAliasFile", command.ErrMalformedAliasFile, CategoryParse},
		{"UnknownProviderInAlias", command.ErrUnknownProviderInAlias, CategoryRegistry},
		{"UnknownModelInAlias", command.ErrUnknownModelInAlias, CategoryRegistry},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Categorize(tc.err); got != tc.want {
				t.Fatalf("Categorize = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestCategorize_NilReturnsUnknown checks the nil contract.
// AC-AMEND-011 (nil path).
func TestCategorize_NilReturnsUnknown(t *testing.T) {
	if got := Categorize(nil); got != CategoryUnknown {
		t.Fatalf("Categorize(nil) = %d, want CategoryUnknown(%d)", got, CategoryUnknown)
	}
}

// TestCategorize_UnrecognizedReturnsUnknown checks the unknown-error fallback.
// AC-AMEND-011 (negative path).
func TestCategorize_UnrecognizedReturnsUnknown(t *testing.T) {
	custom := errors.New("unrelated")
	if got := Categorize(custom); got != CategoryUnknown {
		t.Fatalf("Categorize(custom) = %d, want CategoryUnknown", got)
	}
}

// TestErrorCodeOf_AllSentinelsHaveDistinctCodes ensures no duplicate codes
// in the closed table — protects future additions from accidental shadowing.
// AC-AMEND-004 (governance).
func TestErrorCodeOf_AllSentinelsHaveDistinctCodes(t *testing.T) {
	seen := make(map[string]error, len(sentinelTable))
	for _, e := range sentinelTable {
		if prev, dup := seen[e.code]; dup {
			t.Fatalf("duplicate ErrorCode %q: %v vs %v", e.code, prev, e.sentinel)
		}
		seen[e.code] = e.sentinel
	}
}
