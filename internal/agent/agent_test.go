// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLMProvider is a test double for provider.Provider
type mockLLMProvider struct {
	responses    []string
	errors       []error
	capabilities provider.Capabilities
	callCount    int
}

func (m *mockLLMProvider) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	m.callCount++
	if len(m.errors) > 0 && m.callCount <= len(m.errors) {
		return nil, m.errors[m.callCount-1]
	}

	responseIndex := (m.callCount - 1) % len(m.responses)
	return &provider.CompletionResponse{
		Message: message.Message{
			Role:    "assistant",
			Content: []message.ContentBlock{{Type: "text", Text: m.responses[responseIndex]}},
		},
		Usage: provider.UsageStats{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	return nil, errors.New("streaming not implemented in mock")
}

func (m *mockLLMProvider) Name() string {
	return "mock"
}

func (m *mockLLMProvider) Capabilities() provider.Capabilities {
	if m.capabilities.MaxContextTokens == 0 {
		return provider.Capabilities{
			MaxContextTokens: 4096,
			MaxOutputTokens:  1024,
		}
	}
	return m.capabilities
}

// mockObserver captures OnInteraction calls
type mockObserver struct {
	interactions []*Interaction
	panicOnCall  bool
}

func (m *mockObserver) OnInteraction(ix Interaction) {
	if m.panicOnCall {
		panic("observer panic")
	}
	m.interactions = append(m.interactions, &ix)
}

func TestAgent_Ask_SingleTurn(t *testing.T) {
	// AC-AG-001: Normal single turn
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"Hi!"},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	require.NoError(t, err)

	ctx := context.Background()
	response, err := agent.Ask(ctx, "Hello")

	require.NoError(t, err)
	assert.Equal(t, "Hi!", response)
	assert.Equal(t, 1, mockLLM.callCount)

	// Check history (AC-AG-001: conversation history length 2)
	history := agent.History()
	assert.Len(t, history, 2)
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "Hello", history[0].Content)
	assert.Equal(t, "assistant", history[1].Role)
	assert.Equal(t, "Hi!", history[1].Content)
}

func TestAgent_Ask_SystemPromptNotDuplicated(t *testing.T) {
	// AC-AG-002: System prompt not duplicated in history
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"Hi!", "Hello again!"},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = agent.Ask(ctx, "Hello")
	require.NoError(t, err)

	_, err = agent.Ask(ctx, "Again")
	require.NoError(t, err)

	// Check that LLM was called twice with proper messages
	assert.Equal(t, 2, mockLLM.callCount)

	// Check history - should be 4 messages (2 pairs)
	history := agent.History()
	assert.Len(t, history, 4)

	// Verify no system messages in history
	for _, msg := range history {
		assert.NotEqual(t, "system", msg.Role, "System prompt should not be in history")
	}
}

func TestAgent_Ask_ContextTrim(t *testing.T) {
	// AC-AG-003: Context window trim
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"response"},
		capabilities: provider.Capabilities{
			MaxContextTokens: 100,
			MaxOutputTokens:  50,
		},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	require.NoError(t, err)

	ctx := context.Background()

	// Add enough messages to exceed context
	for i := 0; i < 10; i++ {
		_, err = agent.Ask(ctx, "message")
		require.NoError(t, err)
	}

	// Should have trimmed history to fit
	history := agent.History()
	assert.LessOrEqual(t, len(history), 20) // Should trim old pairs
}

func TestAgent_Ask_LLMError_RollsBackHistory(t *testing.T) {
	// AC-AG-006: LLM error rolls back history
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	modelErr := &provider.ErrModelNotFound{Model: "test-model"}
	mockLLM := &mockLLMProvider{
		errors: []error{modelErr},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = agent.Ask(ctx, "Hello")

	// Error should be wrapped
	assert.Error(t, err)
	assert.True(t, errors.Is(err, modelErr), "Error should wrap ErrModelNotFound")

	var agentErr *AgentError
	assert.ErrorAs(t, err, &agentErr)
	assert.Equal(t, "test-agent", agentErr.AgentName)

	// History should be empty (rolled back)
	history := agent.History()
	assert.Len(t, history, 0, "User message should be rolled back on error")
}

func TestAgent_Ask_InvokesObserver(t *testing.T) {
	// AC-AG-007: Observer invocation
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"Hi!"},
	}

	observer := &mockObserver{
		interactions: make([]*Interaction, 0),
	}

	agent, err := NewAgent(spec, mockLLM, observer)
	require.NoError(t, err)

	ctx := context.Background()
	response, err := agent.Ask(ctx, "X")

	require.NoError(t, err)
	assert.Equal(t, "Hi!", response)

	// Observer should be called exactly once
	assert.Len(t, observer.interactions, 1)
	assert.Equal(t, "X", observer.interactions[0].UserMsg)
	assert.Equal(t, "Hi!", observer.interactions[0].AssistantMsg)
	assert.NoError(t, observer.interactions[0].Err)
}

func TestAgent_Ask_ObserverPanicRecovered(t *testing.T) {
	// AC-AG-008: Observer panic recovered
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"Hi!"},
	}

	observer := &mockObserver{
		interactions: make([]*Interaction, 0),
		panicOnCall:  true,
	}

	agent, err := NewAgent(spec, mockLLM, observer)
	require.NoError(t, err)

	ctx := context.Background()
	response, err := agent.Ask(ctx, "X")

	// Ask should still succeed despite observer panic
	require.NoError(t, err)
	assert.Equal(t, "Hi!", response)

	// History should still be recorded
	history := agent.History()
	assert.Len(t, history, 2)
}

func TestAgent_Load_EmptySystemPrompt(t *testing.T) {
	// AC-AG-004: Empty system prompt rejection during load
	spec := &AgentSpec{
		Name:         "test-agent",
		Model:        "mock/test",
		SystemPrompt: "",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"test"},
	}

	_, err := NewAgent(spec, mockLLM, nil)
	assert.Error(t, err)

	var invalidSpec *ErrInvalidSpec
	assert.ErrorAs(t, err, &invalidSpec)
	assert.Equal(t, "system_prompt", invalidSpec.Field)
}
