// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistry_NewRegistry(t *testing.T) {
	registry := NewRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.agents)
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	// Create a mock agent
	spec := &AgentSpec{
		Name:         "test",
		Model:        "mock/test",
		SystemPrompt: "You are helpful.",
	}

	mockLLM := &mockLLMProvider{
		responses: []string{"test"},
	}

	agent, err := NewAgent(spec, mockLLM, nil)
	assert.NoError(t, err)

	// Register the agent
	registry.Register(agent)

	// Get the agent
	retrieved := registry.Get("test")
	assert.NotNil(t, retrieved)
	assert.Equal(t, "test", retrieved.Name())
}

func TestRegistry_Get_NotFound(t *testing.T) {
	registry := NewRegistry()

	agent := registry.Get("nonexistent")
	assert.Nil(t, agent)
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	// Register multiple agents
	for i := 1; i <= 3; i++ {
		spec := &AgentSpec{
			Name:         "agent" + string(rune('0'+i)),
			Model:        "mock/test",
			SystemPrompt: "You are helpful.",
		}

		mockLLM := &mockLLMProvider{
			responses: []string{"test"},
		}

		agent, err := NewAgent(spec, mockLLM, nil)
		assert.NoError(t, err)
		registry.Register(agent)
	}

	// List should return sorted names
	names := registry.List()
	assert.Len(t, names, 3)
	// Check if sorted (simple check)
	for i := 1; i < len(names); i++ {
		assert.Less(t, names[i-1], names[i])
	}
}

func TestRegistry_List_Empty(t *testing.T) {
	registry := NewRegistry()

	names := registry.List()
	assert.Len(t, names, 0)
}
