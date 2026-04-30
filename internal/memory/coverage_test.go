// Package memory coverage_test exercises sentinel error helpers and BaseProvider
// no-op methods to satisfy the TRUST-Tested 85%+ coverage gate.
package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// TestIsErrUnknownPlugin_Direct verifies the exported helper for the plugin pkg.
func TestIsErrUnknownPlugin_Direct(t *testing.T) {
	t.Parallel()
	if !IsErrUnknownPlugin(ErrUnknownPlugin) {
		t.Error("expected true for direct ErrUnknownPlugin")
	}
	wrapped := fmt.Errorf("layer 1: %w", fmt.Errorf("layer 2: %w", ErrUnknownPlugin))
	if !IsErrUnknownPlugin(wrapped) {
		t.Error("expected true for wrapped ErrUnknownPlugin")
	}
	if IsErrUnknownPlugin(errors.New("unrelated")) {
		t.Error("expected false for unrelated error")
	}
	if IsErrUnknownPlugin(nil) {
		t.Error("expected false for nil")
	}
}

// TestBaseProvider_NoOpMethods_DirectCalls exercises the no-op methods that
// the plugin adapter delegates to its embedded provider; this test covers them
// by invoking BaseProvider directly.
func TestBaseProvider_NoOpMethods_DirectCalls(t *testing.T) {
	t.Parallel()
	var bp BaseProvider

	// QueuePrefetch — must not block or panic
	bp.QueuePrefetch("query", "session")

	// OnTurnStart, OnSessionEnd, OnDelegation — no return values, no panic
	bp.OnTurnStart("session", 1, Message{Role: "user", Content: "hi"})
	bp.OnSessionEnd("session", []Message{{Role: "user", Content: "bye"}})
	bp.OnDelegation("session", "task", "result")
}

// TestErrToolNotHandled_ErrorString verifies the Error() method.
func TestErrToolNotHandled_ErrorString(t *testing.T) {
	t.Parallel()
	err := &ErrToolNotHandled{ToolName: "memory_unknown"}
	got := err.Error()
	want := "tool not handled: memory_unknown"
	if got != want {
		t.Errorf("Error(): got %q want %q", got, want)
	}
}

// TestBaseProvider_HandleToolCall_ReturnsErrToolNotHandled covers the
// fallback HandleToolCall path on BaseProvider.
func TestBaseProvider_HandleToolCall_ReturnsErrToolNotHandled(t *testing.T) {
	t.Parallel()
	var bp BaseProvider
	_, err := bp.HandleToolCall("any_tool", json.RawMessage(`{}`), ToolContext{})
	if err == nil {
		t.Fatal("expected error from BaseProvider.HandleToolCall")
	}
	var tnh *ErrToolNotHandled
	if !errors.As(err, &tnh) {
		t.Errorf("expected *ErrToolNotHandled, got %T: %v", err, err)
	}
}

// TestIsValidProviderName_EdgeCases covers the boundary branches.
func TestIsValidProviderName_EdgeCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		want bool
	}{
		{"", false}, // empty
		{"a", true}, // single char (lower boundary)
		{"abcdefghijklmnopqrstuvwxyz0123_-", true},   // 32 chars (upper boundary, valid)
		{"abcdefghijklmnopqrstuvwxyz0123_-x", false}, // 33 chars (over limit)
		{"1abc", false}, // starts with digit
		{"ABC", false},  // uppercase
		{"a@b", false},  // invalid char
		{"a-b_c", true}, // hyphen and underscore valid
	}
	for _, tc := range cases {
		if got := isValidProviderName(tc.name); got != tc.want {
			t.Errorf("isValidProviderName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
