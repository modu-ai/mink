package llm

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Registry manages LLMProvider instances by name.
// SPEC-GOOSE-LLM-001 §6.7: Name → provider mapping with factory pattern.
//
// @MX:ANCHOR: [AUTO] Registry — provider instance registry and factory
// @MX:REASON: All provider lookups (Get, Default) go through this central registry
type Registry struct {
	mu              sync.RWMutex
	providers       map[string]LLMProvider
	defaultProvider string
	logger          *zap.Logger
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]LLMProvider),
	}
}

// NewRegistryFromConfig creates a Registry from configuration.
// SPEC-GOOSE-LLM-001 §6.7: Bootstrap from LLMConfig.Providers map.
// This is a placeholder for future integration with CONFIG-001.
func NewRegistryFromConfig(config LLMConfig) (*Registry, error) {
	registry := NewRegistry()
	registry.defaultProvider = config.DefaultProvider
	registry.logger = config.Logger

	// Register providers from config
	for name, providerConfig := range config.Providers {
		provider, err := providerConfig.Factory()
		if err != nil {
			return nil, fmt.Errorf("failed to create provider %q: %w", name, err)
		}
		if err := registry.Register(name, provider); err != nil {
			return nil, fmt.Errorf("failed to register provider %q: %w", name, err)
		}
	}

	// Validate default provider exists
	if registry.defaultProvider != "" {
		if _, err := registry.Get(registry.defaultProvider); err != nil {
			return nil, fmt.Errorf("default provider %q not found in registry", registry.defaultProvider)
		}
	}

	return registry, nil
}

// Register registers a provider with the given name.
// Returns error if a provider with the same name already exists.
func (r *Registry) Register(name string, provider LLMProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}

	r.providers[name] = provider
	if r.logger != nil {
		r.logger.Debug("registered LLM provider", zap.String("name", name))
	}

	return nil
}

// Get retrieves a provider by name.
// Returns error if provider is not registered.
func (r *Registry) Get(name string) (LLMProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found in registry", name)
	}

	return provider, nil
}

// Default returns the default provider.
// Returns error if no default is configured or default provider is not registered.
func (r *Registry) Default() (LLMProvider, error) {
	if r.defaultProvider == "" {
		return nil, fmt.Errorf("no default provider configured")
	}
	return r.Get(r.defaultProvider)
}

// SetDefault sets the default provider name.
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("provider %q not found in registry", name)
	}

	r.defaultProvider = name
	return nil
}

// Names returns all registered provider names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// LLMConfig represents the LLM configuration.
// This is a placeholder for future integration with CONFIG-001.
type LLMConfig struct {
	// DefaultProvider is the default provider name.
	DefaultProvider string

	// Providers is a map of provider name to configuration.
	Providers map[string]ProviderConfig

	// Logger is the structured logger.
	Logger *zap.Logger
}

// ProviderConfig represents a single provider configuration.
type ProviderConfig struct {
	// Factory creates the provider instance.
	// This is a function pointer to avoid circular dependencies.
	Factory func() (LLMProvider, error)

	// Kind is the provider kind (e.g., "ollama", "anthropic").
	Kind string
}
