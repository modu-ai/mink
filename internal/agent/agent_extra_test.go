// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgent_Spec(t *testing.T) {
	// REQ-AG-001: Immutable spec reference
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"test"},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	require.NoError(t, err)

	// Spec should return the same reference
	assert.Equal(t, spec, agent.Spec())
}

func TestAgent_Close(t *testing.T) {
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"test"},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	require.NoError(t, err)

	// Close should not error in Phase 0
	err = agent.Close()
	assert.NoError(t, err)
}

func TestAgent_AskStream_NotImplemented(t *testing.T) {
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"test"},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	require.NoError(t, err)

	ctx := context.Background()
	stream, err := agent.AskStream(ctx, "test")

	assert.Error(t, err)
	assert.Nil(t, stream)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestAgent_Name(t *testing.T) {
	spec := &AgentSpec{
		Name:         "my-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"test"},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	require.NoError(t, err)

	assert.Equal(t, "my-agent", agent.Name())
}

func TestValidateSpec_EmptyName(t *testing.T) {
	spec := &AgentSpec{
		Name:         "",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	err := validateSpec(spec)
	assert.Error(t, err)

	var invalidSpec *ErrInvalidSpec
	assert.ErrorAs(t, err, &invalidSpec)
	assert.Equal(t, "name", invalidSpec.Field)
}

func TestValidateSpec_EmptyModel(t *testing.T) {
	spec := &AgentSpec{
		Name:         "test",
		Model:        "",
		SystemPrompt: "You are helpful.",
	}

	err := validateSpec(spec)
	assert.Error(t, err)

	var invalidSpec *ErrInvalidSpec
	assert.ErrorAs(t, err, &invalidSpec)
	assert.Equal(t, "model", invalidSpec.Field)
}

func TestConversation_Truncate(t *testing.T) {
	conv := NewConversation()

	// Add 5 messages
	for i := 0; i < 5; i++ {
		conv.Append(Message{Role: "user", Content: "test"})
	}

	assert.Len(t, conv.Messages(), 5)

	// Truncate to 3
	conv.Truncate(3)

	messages := conv.Messages()
	assert.Len(t, messages, 3)
}

func TestConversation_Truncate_Zero(t *testing.T) {
	conv := NewConversation()

	conv.Append(Message{Role: "user", Content: "test"})

	// Truncate to 0
	conv.Truncate(0)

	messages := conv.Messages()
	assert.Len(t, messages, 0)
}

func TestConversation_Truncate_LargerThanSize(t *testing.T) {
	conv := NewConversation()

	conv.Append(Message{Role: "user", Content: "test"})

	// Truncate to larger size
	conv.Truncate(10)

	messages := conv.Messages()
	assert.Len(t, messages, 1) // Should remain unchanged
}

func TestToolInvoker_Noop(t *testing.T) {
	invoker := &NoopToolInvoker{}

	ctx := context.Background()
	result, err := invoker.Invoke(ctx, "test", "{}")

	assert.NoError(t, err)
	assert.Empty(t, result)
}
