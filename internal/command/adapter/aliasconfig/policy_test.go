// Package aliasconfig — policy.go tests (T-004).
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 AC-AMEND-003
package aliasconfig

import (
	"errors"
	"testing"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/llm/router"
)

// ---------------------------------------------------------------------------
// AC-AMEND-003: ValidateAndPrune — immutable input, cleaned copy returned.
// ---------------------------------------------------------------------------

// TestValidateAndPrune_StrictHappy verifies that all valid entries are returned
// in the cleaned map with zero errors.
func TestValidateAndPrune_StrictHappy(t *testing.T) {
	t.Parallel()
	reg := router.DefaultRegistry()
	m := map[string]string{
		"gpt4":   "openai/gpt-4o",
		"claude": "anthropic/claude-sonnet-4-6",
	}

	cleaned, errs := ValidateAndPrune(m, reg, ValidationPolicy{Strict: true})
	if len(errs) != 0 {
		t.Fatalf("errs = %v, want empty", errs)
	}
	if len(cleaned) != 2 {
		t.Errorf("len(cleaned) = %d, want 2", len(cleaned))
	}
	if cleaned["gpt4"] != "openai/gpt-4o" {
		t.Errorf("cleaned[gpt4] = %q, want %q", cleaned["gpt4"], "openai/gpt-4o")
	}
}

// TestValidateAndPrune_StrictMixedErrors verifies AC-AMEND-003:
// cleaned contains only valid entry; errs contains 2 errors; input m is not mutated.
func TestValidateAndPrune_StrictMixedErrors(t *testing.T) {
	t.Parallel()
	reg := router.DefaultRegistry()
	m := map[string]string{
		"good":  "openai/gpt-4o",
		"bad":   "invalid",
		"empty": "",
	}
	originalLen := len(m)

	cleaned, errs := ValidateAndPrune(m, reg, ValidationPolicy{Strict: true})

	// Cleaned must contain only "good".
	if len(cleaned) != 1 {
		t.Errorf("len(cleaned) = %d, want 1", len(cleaned))
	}
	if cleaned["good"] != "openai/gpt-4o" {
		t.Errorf("cleaned[good] = %q, want %q", cleaned["good"], "openai/gpt-4o")
	}
	// Errors: "bad" (ErrInvalidCanonical) + "empty" (ErrEmptyAliasEntry) = 2.
	if len(errs) != 2 {
		t.Errorf("len(errs) = %d, want 2", len(errs))
	}
	// Input map must not be mutated.
	if len(m) != originalLen {
		t.Errorf("input map mutated: len(m) = %d, want %d", len(m), originalLen)
	}
}

// TestValidateAndPrune_InputNotMutated is an explicit mutation guard.
func TestValidateAndPrune_InputNotMutated(t *testing.T) {
	t.Parallel()
	reg := router.DefaultRegistry()
	m := map[string]string{
		"good": "openai/gpt-4o",
		"bad":  "invalid",
	}
	// Snapshot the original keys.
	origKeys := make(map[string]string, len(m))
	for k, v := range m {
		origKeys[k] = v
	}

	ValidateAndPrune(m, reg, ValidationPolicy{Strict: false})

	// Verify no keys were removed from m.
	for k, v := range origKeys {
		if m[k] != v {
			t.Errorf("m[%q] changed: got %q, want %q", k, m[k], v)
		}
	}
	if len(m) != len(origKeys) {
		t.Errorf("m length changed: %d → %d", len(origKeys), len(m))
	}
}

// TestValidateAndPrune_NilInputReturnsEmpty verifies nil input produces empty cleaned map.
func TestValidateAndPrune_NilInputReturnsEmpty(t *testing.T) {
	t.Parallel()
	cleaned, errs := ValidateAndPrune(nil, nil, ValidationPolicy{})
	if len(errs) != 0 {
		t.Errorf("errs = %v, want empty", errs)
	}
	if cleaned == nil {
		t.Error("cleaned is nil, want empty map")
	}
}

// TestValidateAndPrune_LenientPreservesValidEntries verifies lenient mode (Strict=false)
// still prunes format-invalid entries.
func TestValidateAndPrune_LenientPreservesValidEntries(t *testing.T) {
	t.Parallel()
	reg := router.DefaultRegistry()
	m := map[string]string{
		"good": "openai/gpt-4o",
		"bad":  "no-slash-here",
	}

	cleaned, errs := ValidateAndPrune(m, reg, ValidationPolicy{Strict: false})
	if len(cleaned) != 1 {
		t.Errorf("len(cleaned) = %d, want 1", len(cleaned))
	}
	if cleaned["good"] != "openai/gpt-4o" {
		t.Errorf("cleaned[good] = %q, want %q", cleaned["good"], "openai/gpt-4o")
	}
	if len(errs) != 1 {
		t.Errorf("len(errs) = %d, want 1", len(errs))
	}
}

// TestValidateAndPrune_AggregateAsJoin verifies that AggregateAsJoin=true returns
// a single error wrapping all individual errors.
func TestValidateAndPrune_AggregateAsJoin(t *testing.T) {
	t.Parallel()
	reg := router.DefaultRegistry()
	m := map[string]string{
		"bad1": "invalid1",
		"bad2": "invalid2",
	}

	_, errs := ValidateAndPrune(m, reg, ValidationPolicy{AggregateAsJoin: true})
	if len(errs) != 1 {
		t.Errorf("AggregateAsJoin: len(errs) = %d, want 1", len(errs))
	}
	// The aggregated error must wrap the individual sentinels.
	if !errors.Is(errs[0], command.ErrInvalidCanonical) {
		t.Errorf("aggregated error does not wrap ErrInvalidCanonical: %v", errs[0])
	}
}

// TestValidateAndPrune_MaxErrors verifies that MaxErrors caps the error slice.
func TestValidateAndPrune_MaxErrors(t *testing.T) {
	t.Parallel()
	reg := router.DefaultRegistry()
	m := map[string]string{
		"bad1": "a",
		"bad2": "b",
		"bad3": "c",
		"good": "openai/gpt-4o",
	}

	_, errs := ValidateAndPrune(m, reg, ValidationPolicy{MaxErrors: 1})
	if len(errs) > 1 {
		t.Errorf("len(errs) = %d, want <= 1 (MaxErrors=1)", len(errs))
	}
}
