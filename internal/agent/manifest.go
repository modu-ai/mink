// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AgentSpec defines the agent's persona and configuration from YAML.
// @MX:ANCHOR: [AUTO] Agent configuration structure
// @MX:REASON: Loaded from YAML files, consumed by NewAgent, used by Registry
type AgentSpec struct {
	// Name is the unique identifier for this agent
	Name string `yaml:"name"`
	// Version is the spec version
	Version int `yaml:"version"`
	// Model is the model identifier in "provider/model" format
	Model string `yaml:"model"`
	// SystemPrompt is the persona's system prompt
	SystemPrompt string `yaml:"system_prompt"`
	// Examples are few-shot examples (optional)
	Examples []Example `yaml:"examples,omitempty"`
}

// Example represents a few-shot conversation example.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Few-shot examples for persona behavior
type Example struct {
	User      string `yaml:"user"`
	Assistant string `yaml:"assistant"`
}

// LoadAgentSpec loads an AgentSpec from a YAML file.
// @MX:ANCHOR: [AUTO] YAML loader for agent specs
// @MX:REASON: Entry point for loading agent configurations from disk
func LoadAgentSpec(path string) (*AgentSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read spec file: %w", err)
	}

	var spec AgentSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse spec YAML: %w", err)
	}

	// Validate spec
	if err := validateSpec(&spec); err != nil {
		return nil, err
	}

	return &spec, nil
}

// validateSpec checks if the spec meets minimum requirements.
// Implements REQ-AG-009: Empty system prompt rejection.
func validateSpec(spec *AgentSpec) error {
	if strings.TrimSpace(spec.SystemPrompt) == "" {
		return &ErrInvalidSpec{
			Field:  "system_prompt",
			Reason: "must not be empty",
		}
	}

	if strings.TrimSpace(spec.Name) == "" {
		return &ErrInvalidSpec{
			Field:  "name",
			Reason: "must not be empty",
		}
	}

	if strings.TrimSpace(spec.Model) == "" {
		return &ErrInvalidSpec{
			Field:  "model",
			Reason: "must not be empty",
		}
	}

	return nil
}

// ParseModel splits the model string into provider and model components.
// Returns (provider, model).
// @MX:ANCHOR: [AUTO] Model string parser for "provider/model" format
// @MX:REASON: Used by agent initialization to lookup LLM provider from registry
func (s *AgentSpec) ParseModel() (string, string) {
	parts := strings.SplitN(s.Model, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// Default: treat entire string as model, assume first available provider
	return "", s.Model
}
