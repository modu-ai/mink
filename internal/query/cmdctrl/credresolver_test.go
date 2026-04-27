// Package cmdctrl provides tests for credential pool resolution support.
//
// SPEC: SPEC-GOOSE-CMDCTX-CREDPOOL-WIRE-001
package cmdctrl

import (
	"errors"
	"fmt"
	"testing"
)

// TestExtractProvider verifies the extractProvider function correctly extracts
// the provider identifier from model IDs.
//
// AC-CCWIRE-003: Provider extraction via strings.SplitN.
func TestExtractProvider(t *testing.T) {
	tests := []struct {
		name     string
		modelID  string
		expected string
	}{
		{
			name:     "standard model ID with provider and model",
			modelID:  "openai/gpt-4",
			expected: "openai",
		},
		{
			name:     "anthropic model ID",
			modelID:  "anthropic/claude-3-opus-20240229",
			expected: "anthropic",
		},
		{
			name:     "model ID with multiple slashes",
			modelID:  "provider/org/model",
			expected: "provider",
		},
		{
			name:     "empty string returns empty",
			modelID:  "",
			expected: "",
		},
		{
			name:     "no slash returns empty",
			modelID:  "gpt-4",
			expected: "",
		},
		{
			name:     "only slash returns empty",
			modelID:  "/",
			expected: "",
		},
		{
			name:     "slash at end returns first part",
			modelID:  "openai/",
			expected: "openai",
		},
		{
			name:     "complex model path",
			modelID:  "google/models/gemini-pro",
			expected: "google",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractProvider(tt.modelID)
			if result != tt.expected {
				t.Errorf("extractProvider(%q) = %q, want %q", tt.modelID, result, tt.expected)
			}
		})
	}
}

// TestCredentialPoolResolverInterface verifies that fakeResolver implements
// the CredentialPoolResolver interface correctly.
//
// AC-CCWIRE-001: Interface compliance.
func TestCredentialPoolResolverInterface(t *testing.T) {
	// fakeResolver is defined in controller_test.go
	// This test verifies interface compliance at compile time
	var _ CredentialPoolResolver = (*fakeResolver)(nil)
}

// TestErrCredentialUnavailableIsSentinel verifies that ErrCredentialUnavailable
// is a proper sentinel error compatible with errors.Is.
//
// AC-CCWIRE-004: Sentinel error errors.Is compatibility.
func TestErrCredentialUnavailableIsSentinel(t *testing.T) {
	// Verify direct comparison works
	if !errors.Is(ErrCredentialUnavailable, ErrCredentialUnavailable) {
		t.Error("ErrCredentialUnavailable should match itself via errors.Is")
	}

	// Create a wrapped error using fmt.Errorf
	wrappedErr := errors.New("wrapped: " + ErrCredentialUnavailable.Error())

	// This should NOT match since we're not actually wrapping
	if errors.Is(wrappedErr, ErrCredentialUnavailable) {
		t.Error("String concatenation should not create a wrapped error")
	}

	// Proper wrapping with fmt.Errorf
	wrappedErr2 := fmt.Errorf("context: %w", ErrCredentialUnavailable)
	if !errors.Is(wrappedErr2, ErrCredentialUnavailable) {
		t.Error("ErrCredentialUnavailable should be detectable via errors.Is when properly wrapped")
	}
}
