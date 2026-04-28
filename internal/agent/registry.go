// Package agent provides the AI agent runtime with persona management and conversation history.
// SPEC-GOOSE-AGENT-001
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"go.uber.org/zap"
)

// Registry manages loaded agent instances by name.
// @MX:ANCHOR: [AUTO] Agent registry for name-to-instance lookup
// @MX:REASON: Single-process scope agent management, prevents duplicate loads
type Registry struct {
	agents map[string]Agent
	logger *zap.Logger
}

// NewRegistry creates a new empty agent registry.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Factory function for Registry initialization
func NewRegistry() *Registry {
	logger, _ := zap.NewDevelopment()
	return &Registry{
		agents: make(map[string]Agent),
		logger: logger,
	}
}

// Register adds an agent to the registry.
// If an agent with the same name already exists, it is replaced.
// @MX:ANCHOR: [AUTO] Agent registration method
// @MX:REASON: Called by LoadFromDir after agent creation
func (r *Registry) Register(agent Agent) {
	r.agents[agent.Name()] = agent
}

// Get retrieves an agent by name.
// Returns nil if not found.
// @MX:ANCHOR: [AUTO] Agent lookup method
// @MX:REASON: Called by CLI to get agent instances by name
func (r *Registry) Get(name string) Agent {
	return r.agents[name]
}

// List returns all registered agent names.
// @MX:NOTE: [SPEC-GOOSE-AGENT-001] Returns sorted list for consistent ordering
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LoadFromDir loads all agent specs from a directory and creates agent instances.
// Implements loading from $GOOSE_HOME/agents/*.yaml.
// @MX:ANCHOR: [AUTO] Bulk agent loader from directory
// @MX:REASON: Entry point for loading all agents at startup
func (r *Registry) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read agents directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Load only .yaml files
		if filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}

		specPath := filepath.Join(dir, entry.Name())
		spec, err := LoadAgentSpec(specPath)
		if err != nil {
			r.logger.Warn("failed to load agent spec",
				zap.String("path", specPath),
				zap.Error(err),
			)
			continue
		}

		// TODO: Resolve provider from registry
		// For now, we can't create agents without LLM provider
		r.logger.Debug("loaded agent spec",
			zap.String("name", spec.Name),
			zap.String("model", spec.Model),
		)

		// Store spec for later instantiation
		// TODO: This is a placeholder - real implementation would create agents
		_ = spec
	}

	return nil
}
