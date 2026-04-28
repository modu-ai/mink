package llm

import (
	"testing"
)

// TestRegistry_GetOllama validates AC-LLM-007: Registry lookup.
func TestRegistry_GetOllama(t *testing.T) {
	// Given: config with mock provider (avoiding ollama import cycle)
	config := LLMConfig{
		DefaultProvider: "test",
		Providers: map[string]ProviderConfig{
			"test": {
				Kind: "test",
				Factory: func() (LLMProvider, error) {
					return &mockLLMProvider{name: "test"}, nil
				},
			},
		},
	}

	// When: create registry from config
	registry, err := NewRegistryFromConfig(config)
	if err != nil {
		t.Fatalf("NewRegistryFromConfig: %v", err)
	}

	// Then: verify provider lookup
	provider, err := registry.Get("test")
	if err != nil {
		t.Fatalf("Get test: %v", err)
	}

	if provider.Name() != "test" {
		t.Errorf("expected provider name 'test', got %q", provider.Name())
	}

	// Verify default provider
	defaultProvider, err := registry.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}

	if defaultProvider.Name() != "test" {
		t.Errorf("expected default provider 'test', got %q", defaultProvider.Name())
	}
}

// TestRegistry_RegisterDuplicate validates error on duplicate registration.
func TestRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewRegistry()

	provider := &mockLLMProvider{name: "test"}

	err := registry.Register("test", provider)
	if err != nil {
		t.Fatalf("first Register: %v", err)
	}

	// Second registration should fail
	err = registry.Register("test", provider)
	if err == nil {
		t.Error("expected error on duplicate registration")
	}
}

// TestRegistry_GetNotFound validates error when provider not found.
func TestRegistry_GetNotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent provider")
	}
}

// TestRegistry_DefaultNotConfigured validates error when no default set.
func TestRegistry_DefaultNotConfigured(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Default()
	if err == nil {
		t.Error("expected error when no default configured")
	}
}

// TestRegistry_DefaultNotFound validates error when default provider not registered.
func TestRegistry_DefaultNotFound(t *testing.T) {
	config := LLMConfig{
		DefaultProvider: "nonexistent",
		Providers:      map[string]ProviderConfig{},
	}

	_, err := NewRegistryFromConfig(config)
	if err == nil {
		t.Error("expected error when default provider not found")
	}
}

// TestRegistry_SetDefault validates setting default provider.
func TestRegistry_SetDefault(t *testing.T) {
	registry := NewRegistry()

	provider := &mockLLMProvider{name: "test"}
	err := registry.Register("test", provider)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	err = registry.SetDefault("test")
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	// Verify default is set
	defaultProvider, err := registry.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}

	if defaultProvider.Name() != "test" {
		t.Errorf("expected default 'test', got %q", defaultProvider.Name())
	}
}

