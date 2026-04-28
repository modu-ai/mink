// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPersona_WithSystemPrompt(t *testing.T) {
	// REQ-AG-003: System prompt is first
	spec := &AgentSpec{
		Name:         "test",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	messages := BuildPersona(spec)

	assert.Len(t, messages, 1)
	assert.Equal(t, "system", messages[0].Role)
	assert.Contains(t, messages[0].Content, "helpful")
}

func TestBuildPersona_WithExamples(t *testing.T) {
	// REQ-AG-013: Few-shot examples after system prompt
	spec := &AgentSpec{
		Name:         "test",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
		Examples: []Example{
			{User: "Hello", Assistant: "Hi"},
			{User: "Bye", Assistant: "Goodbye"},
		},
	}

	messages := BuildPersona(spec)

	// System + 2 examples (4 messages: system, user, assistant, user, assistant)
	assert.Len(t, messages, 5)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
	assert.Equal(t, "Hello", messages[1].Content)
	assert.Equal(t, "assistant", messages[2].Role)
	assert.Equal(t, "Hi", messages[2].Content)
	assert.Equal(t, "user", messages[3].Role)
	assert.Equal(t, "Bye", messages[3].Content)
	assert.Equal(t, "assistant", messages[4].Role)
	assert.Equal(t, "Goodbye", messages[4].Content)
}

func TestBuildPersona_NoExamples(t *testing.T) {
	spec := &AgentSpec{
		Name:         "test",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	messages := BuildPersona(spec)

	assert.Len(t, messages, 1)
	assert.Equal(t, "system", messages[0].Role)
}
