package credproxy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// ProviderStore manages provider credential references (NOT secrets themselves).
// REQ-CREDPROXY-001: References stored in ~/.goose/secrets/providers.yaml
//
// @MX:ANCHOR: [AUTO] Provider reference store
// @MX:REASON: Central credential configuration used by proxy and client, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-001, REQ-CREDPROXY-004
type ProviderStore struct {
	mu        sync.RWMutex
	path      string
	providers map[string]ProviderRef
}

// ProviderRef contains a reference to a credential stored in the keyring.
// The actual secret value is NEVER stored here - only the keyring ID.
//
// REQ-CREDPROXY-004: Scoped credential binding — secret only injected for matching host patterns
type ProviderRef struct {
	KeyringID   string `yaml:"keyring_id"`   // ID used to retrieve secret from keyring
	HostPattern string `yaml:"host_pattern"` // Glob pattern for scope binding (REQ-CREDPROXY-004)
	Provider    string `yaml:"provider"`     // Provider name (e.g., "openai", "anthropic")
}

// providerStoreFile is the structure of the providers.yaml file.
type providerStoreFile struct {
	Providers map[string]ProviderRef `yaml:"providers"`
}

// NewProviderStore creates a new provider store.
// The store is loaded from the specified path.
//
// REQ-CREDPROXY-001: References stored in ~/.goose/secrets/providers.yaml
func NewProviderStore(path string) (*ProviderStore, error) {
	if path == "" {
		return nil, fmt.Errorf("store path cannot be empty")
	}

	ps := &ProviderStore{
		path:      path,
		providers: make(map[string]ProviderRef),
	}

	// Load existing store if it exists
	if err := ps.load(); err != nil {
		return nil, fmt.Errorf("failed to load provider store: %w", err)
	}

	return ps, nil
}

// Add adds a provider reference to the store.
//
// @MX:ANCHOR: [AUTO] Add provider reference
// @MX:REASON: Used by CLI and tests, fan_in >= 3
func (ps *ProviderStore) Add(provider string, ref ProviderRef) error {
	if provider == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if ref.KeyringID == "" {
		return fmt.Errorf("keyring ID cannot be empty")
	}
	if ref.HostPattern == "" {
		return fmt.Errorf("host pattern cannot be empty")
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.providers[provider] = ref

	// Persist to disk
	if err := ps.save(); err != nil {
		return fmt.Errorf("failed to save provider store: %w", err)
	}

	return nil
}

// Get retrieves a provider reference by name.
func (ps *ProviderStore) Get(provider string) (ProviderRef, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	ref, exists := ps.providers[provider]
	return ref, exists
}

// List returns all provider references.
func (ps *ProviderStore) List() map[string]ProviderRef {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]ProviderRef, len(ps.providers))
	for k, v := range ps.providers {
		result[k] = v
	}
	return result
}

// Remove removes a provider reference from the store.
func (ps *ProviderStore) Remove(provider string) error {
	if provider == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if _, exists := ps.providers[provider]; !exists {
		return fmt.Errorf("provider not found: %s", provider)
	}

	delete(ps.providers, provider)

	// Persist to disk
	if err := ps.save(); err != nil {
		return fmt.Errorf("failed to save provider store: %w", err)
	}

	return nil
}

// MatchForHost finds the provider reference that matches the given host.
// This implements REQ-CREDPROXY-004: Scoped credential binding.
//
// @MX:ANCHOR: [AUTO] Host pattern matching
// @MX:REASON: Core security enforcement for scoped credential binding, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-004, AC-CREDPROXY-03
func (ps *ProviderStore) MatchForHost(host string) (string, ProviderRef, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Iterate through all providers to find a match
	for provider, ref := range ps.providers {
		if matchHostPattern(ref.HostPattern, host) {
			return provider, ref, nil
		}
	}

	return "", ProviderRef{}, fmt.Errorf("no provider found for host: %s", host)
}

// load loads the provider store from disk.
func (ps *ProviderStore) load() error {
	// Check if file exists
	if _, err := os.Stat(ps.path); os.IsNotExist(err) {
		// File doesn't exist yet, which is fine
		return nil
	}

	// Read file
	data, err := os.ReadFile(ps.path)
	if err != nil {
		return fmt.Errorf("failed to read provider store: %w", err)
	}

	// Parse YAML
	var storeFile providerStoreFile
	if err := yaml.Unmarshal(data, &storeFile); err != nil {
		return fmt.Errorf("failed to parse provider store: %w", err)
	}

	// Load providers
	ps.providers = storeFile.Providers
	if ps.providers == nil {
		ps.providers = make(map[string]ProviderRef)
	}

	return nil
}

// save saves the provider store to disk.
func (ps *ProviderStore) save() error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(ps.path), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Marshal to YAML
	storeFile := providerStoreFile{
		Providers: ps.providers,
	}
	data, err := yaml.Marshal(storeFile)
	if err != nil {
		return fmt.Errorf("failed to marshal provider store: %w", err)
	}

	// Write to file
	if err := os.WriteFile(ps.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write provider store: %w", err)
	}

	return nil
}
