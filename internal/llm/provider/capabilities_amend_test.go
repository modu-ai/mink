package provider_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/stretchr/testify/assert"
)

// TestCapabilities_NewFields verifies that the two new capability fields exist
// on the Capabilities struct and default to false (zero value).
// RED: fails until JSONMode and UserID fields are added to provider.Capabilities.
// AC-AMEND-001 (partial).
func TestCapabilities_NewFields(t *testing.T) {
	t.Parallel()

	// zero-value struct — both new fields must default to false
	zero := provider.Capabilities{}
	assert.False(t, zero.JSONMode, "JSONMode must default to false")
	assert.False(t, zero.UserID, "UserID must default to false")

	// explicit true assignment
	caps := provider.Capabilities{
		JSONMode: true,
		UserID:   true,
	}
	assert.True(t, caps.JSONMode, "JSONMode must be settable to true")
	assert.True(t, caps.UserID, "UserID must be settable to true")
}

// TestCapabilities_ExistingFieldsUnchanged verifies that the pre-existing fields
// are still accessible with the same semantics after the amendment adds new ones.
// Regression guard for REQ-AMEND-001.
func TestCapabilities_ExistingFieldsUnchanged(t *testing.T) {
	t.Parallel()

	caps := provider.Capabilities{
		Streaming:        true,
		Tools:            true,
		Vision:           true,
		Embed:            false,
		AdaptiveThinking: true,
		MaxContextTokens: 200000,
		MaxOutputTokens:  8192,
	}

	assert.True(t, caps.Streaming)
	assert.True(t, caps.Tools)
	assert.True(t, caps.Vision)
	assert.False(t, caps.Embed)
	assert.True(t, caps.AdaptiveThinking)
	assert.Equal(t, 200000, caps.MaxContextTokens)
	assert.Equal(t, 8192, caps.MaxOutputTokens)

	// new fields must also be accessible (zero value)
	assert.False(t, caps.JSONMode)
	assert.False(t, caps.UserID)
}
