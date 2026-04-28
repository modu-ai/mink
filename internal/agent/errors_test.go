// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"errors"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/stretchr/testify/assert"
)

func TestErrInvalidSpec(t *testing.T) {
	// AC-AG-004: Empty system prompt rejection
	err := &ErrInvalidSpec{Field: "system_prompt", Reason: "must not be empty"}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "system_prompt")
	assert.Contains(t, err.Error(), "must not be empty")
}

func TestAgentError_Wrapping(t *testing.T) {
	// AC-AG-006: LLM error wrapping
	originalErr := &provider.ErrModelNotFound{Model: "test-model"}
	wrappedErr := &AgentError{
		AgentName: "test-agent",
		Cause:     originalErr,
	}

	assert.Error(t, wrappedErr)
	assert.True(t, errors.Is(wrappedErr, originalErr))
	assert.Contains(t, wrappedErr.Error(), "test-agent")
}

func TestErrAgentNotReady(t *testing.T) {
	// REQ-AG-007: Agent not ready state error
	err := &ErrAgentNotReady{AgentName: "test-agent", State: "Loading"}
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "test-agent")
	assert.Contains(t, err.Error(), "Loading")
}
