// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAgentSpec_ValidSpec(t *testing.T) {
	// AC-AG-001: Valid spec loading
	yamlContent := `
name: test-agent
version: 1
model: ollama/qwen2.5:3b
system_prompt: |
  You are a helpful assistant.
examples:
  - user: "Hello"
    assistant: "Hi there!"
`
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(yamlContent), 0644))

	spec, err := LoadAgentSpec(specPath)
	require.NoError(t, err)

	assert.Equal(t, "test-agent", spec.Name)
	assert.Equal(t, 1, spec.Version)
	assert.Equal(t, "ollama/qwen2.5:3b", spec.Model)
	assert.Contains(t, spec.SystemPrompt, "helpful assistant")
	assert.Len(t, spec.Examples, 1)
	assert.Equal(t, "Hello", spec.Examples[0].User)
	assert.Equal(t, "Hi there!", spec.Examples[0].Assistant)
}

func TestLoadAgentSpec_EmptySystemPrompt(t *testing.T) {
	// AC-AG-004: Empty system prompt rejection
	yamlContent := `
name: test-agent
version: 1
model: ollama/qwen2.5:3b
system_prompt: ""
`
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(yamlContent), 0644))

	_, err := LoadAgentSpec(specPath)
	assert.Error(t, err)

	var invalidSpec *ErrInvalidSpec
	assert.ErrorAs(t, err, &invalidSpec)
	assert.Equal(t, "system_prompt", invalidSpec.Field)
}

func TestLoadAgentSpec_InvalidYAML(t *testing.T) {
	yamlContent := `
name: test-agent
invalid yaml: [
`
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "test.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte(yamlContent), 0644))

	_, err := LoadAgentSpec(specPath)
	assert.Error(t, err)
}

func TestLoadAgentSpec_FileNotFound(t *testing.T) {
	_, err := LoadAgentSpec("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestParseModelString(t *testing.T) {
	tests := []struct {
		name          string
		modelStr      string
		expectedProv  string
		expectedModel string
	}{
		{"Ollama model", "ollama/qwen2.5:3b", "ollama", "qwen2.5:3b"},
		{"OpenAI model", "openai/gpt-4", "openai", "gpt-4"},
		{"Anthropic model", "anthropic/claude-3-opus", "anthropic", "claude-3-opus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &AgentSpec{Model: tt.modelStr}
			provider, model := spec.ParseModel()
			assert.Equal(t, tt.expectedProv, provider)
			assert.Equal(t, tt.expectedModel, model)
		})
	}
}
