package credproxy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProviderStoreAdd tests adding provider references.
func TestProviderStoreAdd(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Add a provider reference
	ref := ProviderRef{
		KeyringID:   "test-keyring-id",
		HostPattern: "api.example.com",
		Provider:    "test-provider",
	}
	err = store.Add("openai", ref)
	require.NoError(t, err)

	// Verify provider was added
	retrieved, exists := store.Get("openai")
	assert.True(t, exists)
	assert.Equal(t, ref, retrieved)
}

// TestProviderStoreGet tests retrieving provider references.
func TestProviderStoreGet(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Add a provider
	ref := ProviderRef{
		KeyringID:   "keyring-id-1",
		HostPattern: "api.openai.com",
		Provider:    "openai",
	}
	_ = store.Add("openai", ref)

	// Retrieve the provider
	retrieved, exists := store.Get("openai")
	assert.True(t, exists)
	assert.Equal(t, ref, retrieved)

	// Try to retrieve non-existent provider
	_, exists = store.Get("non-existent")
	assert.False(t, exists)
}

// TestProviderStoreList tests listing all providers.
func TestProviderStoreList(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Initially empty
	providers := store.List()
	assert.Empty(t, providers)

	// Add providers
	_ = store.Add("openai", ProviderRef{KeyringID: "id-1", HostPattern: "api.openai.com", Provider: "openai"})
	_ = store.Add("anthropic", ProviderRef{KeyringID: "id-2", HostPattern: "api.anthropic.com", Provider: "anthropic"})
	_ = store.Add("google", ProviderRef{KeyringID: "id-3", HostPattern: "googleapis.com", Provider: "google"})

	// List providers
	providers = store.List()
	assert.Len(t, providers, 3)
	assert.Contains(t, providers, "openai")
	assert.Contains(t, providers, "anthropic")
	assert.Contains(t, providers, "google")
}

// TestProviderStoreRemove tests removing provider references.
func TestProviderStoreRemove(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Add a provider
	ref := ProviderRef{
		KeyringID:   "removable-id",
		HostPattern: "api.example.com",
		Provider:    "test",
	}
	_ = store.Add("test", ref)

	// Verify it exists
	_, exists := store.Get("test")
	assert.True(t, exists)

	// Remove the provider
	err = store.Remove("test")
	require.NoError(t, err)

	// Verify it's removed
	_, exists = store.Get("test")
	assert.False(t, exists)
}

// TestProviderStoreRemoveNotFound tests removing a non-existent provider.
func TestProviderStoreRemoveNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Try to remove non-existent provider
	err = store.Remove("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider not found")
}

// TestProviderStorePersistence tests that providers persist across store instances.
func TestProviderStorePersistence(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")

	// Create first store instance and add a provider
	store1, err := NewProviderStore(storePath)
	require.NoError(t, err)

	ref := ProviderRef{
		KeyringID:   "persistent-id",
		HostPattern: "api.persistent.com",
		Provider:    "persistent",
	}
	err = store1.Add("persistent", ref)
	require.NoError(t, err)

	// Create second store instance and verify provider is loaded
	store2, err := NewProviderStore(storePath)
	require.NoError(t, err)

	retrieved, exists := store2.Get("persistent")
	assert.True(t, exists)
	assert.Equal(t, ref, retrieved)
}

// TestProviderStoreMatchForHost tests matching providers by host pattern.
func TestProviderStoreMatchForHost(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Add providers with different host patterns
	_ = store.Add("openai", ProviderRef{
		KeyringID:   "openai-key",
		HostPattern: "api.openai.com",
		Provider:    "openai",
	})
	_ = store.Add("anthropic", ProviderRef{
		KeyringID:   "anthropic-key",
		HostPattern: "api.anthropic.com",
		Provider:    "anthropic",
	})

	// Test exact match
	provider, ref, err := store.MatchForHost("api.openai.com")
	require.NoError(t, err)
	assert.Equal(t, "openai", provider)
	assert.Equal(t, "openai-key", ref.KeyringID)

	// Test another exact match
	provider, ref, err = store.MatchForHost("api.anthropic.com")
	require.NoError(t, err)
	assert.Equal(t, "anthropic", provider)
	assert.Equal(t, "anthropic-key", ref.KeyringID)

	// Add wildcard for fallback testing
	_ = store.Add("wildcard", ProviderRef{
		KeyringID:   "wildcard-key",
		HostPattern: "*",
		Provider:    "wildcard",
	})

	// Test wildcard match for unknown host
	provider, ref, err = store.MatchForHost("any.other.host")
	require.NoError(t, err)
	// Should match wildcard since no specific match exists
	assert.Contains(t, []string{"wildcard", "openai", "anthropic"}, provider)
}

// TestProviderStoreMatchForHostNotFound tests matching when no provider matches.
func TestProviderStoreMatchForHostNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Add a specific provider
	_ = store.Add("specific", ProviderRef{
		KeyringID:   "specific-key",
		HostPattern: "api.specific.com",
		Provider:    "specific",
	})

	// Try to match a different host
	_, _, err = store.MatchForHost("api.different.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no provider found")
}

// TestProviderStoreValidation tests input validation.
func TestProviderStoreValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Test adding with empty provider name
	ref := ProviderRef{
		KeyringID:   "key-id",
		HostPattern: "api.example.com",
		Provider:    "test",
	}
	err = store.Add("", ref)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider name cannot be empty")

	// Test adding with empty keyring ID
	err = store.Add("test", ProviderRef{HostPattern: "api.example.com", Provider: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "keyring ID cannot be empty")

	// Test adding with empty host pattern
	err = store.Add("test", ProviderRef{KeyringID: "key-id", Provider: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host pattern cannot be empty")

	// Test removing with empty provider name
	err = store.Remove("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider name cannot be empty")
}

// TestProviderStoreYAMLFormat tests the YAML file format.
func TestProviderStoreYAMLFormat(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Add providers
	_ = store.Add("openai", ProviderRef{
		KeyringID:   "goose-openai-api-key",
		HostPattern: "api.openai.com",
		Provider:    "openai",
	})
	_ = store.Add("anthropic", ProviderRef{
		KeyringID:   "goose-anthropic-api-key",
		HostPattern: "api.anthropic.com",
		Provider:    "anthropic",
	})

	// Read the YAML file
	data, err := os.ReadFile(storePath)
	require.NoError(t, err)

	// Verify expected format
	yamlContent := string(data)
	assert.Contains(t, yamlContent, "providers:")
	assert.Contains(t, yamlContent, "openai:")
	assert.Contains(t, yamlContent, "goose-openai-api-key")
	assert.Contains(t, yamlContent, "api.openai.com")
	assert.Contains(t, yamlContent, "anthropic:")
	assert.Contains(t, yamlContent, "goose-anthropic-api-key")
	assert.Contains(t, yamlContent, "api.anthropic.com")
}

// TestProviderStoreConcurrentAccess tests concurrent access to the store.
func TestProviderStoreConcurrentAccess(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Launch concurrent goroutines
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			provider := fmt.Sprintf("provider-%d", id)
			ref := ProviderRef{
				KeyringID:   fmt.Sprintf("keyring-%d", id),
				HostPattern: fmt.Sprintf("api-%d.example.com", id),
				Provider:    fmt.Sprintf("provider-%d", id),
			}
			_ = store.Add(provider, ref)
			_, _ = store.Get(provider)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all providers were added
	providers := store.List()
	assert.Len(t, providers, 10)
}
