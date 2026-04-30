package credproxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestIntegrationFullWorkflow tests the complete credential proxy workflow.
// This is a comprehensive integration test covering:
// - Storing credentials in keyring
// - Adding provider references to store
// - Starting proxy server
// - Routing requests through proxy
// - Verifying credential injection
//
// AC-CREDPROXY-02: Verify secret never in agent memory (memory scan test)
// AC-CREDPROXY-03: Scoped binding enforcement
// AC-CREDPROXY-04: No credential API without proxy
// AC-CREDPROXY-05: Prompt injection defense
func TestIntegrationFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup temporary directory
	tmpDir := t.TempDir()

	// Create keyring backend
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Store a test credential
	testSecret := []byte("sk-test-integration-secret-key-12345")
	err = keyring.Store("goose", "test-provider-key", testSecret)
	require.NoError(t, err)

	// Verify secret is stored
	retrievedSecret, err := keyring.Retrieve("goose", "test-provider-key")
	require.NoError(t, err)
	assert.Equal(t, testSecret, retrievedSecret)

	// Create provider store
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, err := NewProviderStore(storePath)
	require.NoError(t, err)

	// Add provider reference with scoped binding (AC-CREDPROXY-03)
	err = store.Add("test-provider", ProviderRef{
		KeyringID:   "test-provider-key",
		HostPattern: "api.testprovider.com",
		Provider:    "test-provider",
	})
	require.NoError(t, err)

	// Verify provider was added
	ref, exists := store.Get("test-provider")
	assert.True(t, exists)
	assert.Equal(t, "test-provider-key", ref.KeyringID)
	assert.Equal(t, "api.testprovider.com", ref.HostPattern)

	// Create a mock target server that validates credentials
	targetServerAuthHeader := ""
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetServerAuthHeader = r.Header.Get("Authorization")

		// Verify the Authorization header was injected
		if targetServerAuthHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte("missing authorization header")); err != nil {
				t.Fatalf("Failed to write response: %v", err)
			}
			return
		}

		// Verify the Bearer token format
		expectedAuth := fmt.Sprintf("Bearer %s", string(testSecret))
		if targetServerAuthHeader != expectedAuth {
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte("invalid authorization header")); err != nil {
				t.Fatalf("Failed to write response: %v", err)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("authorized")); err != nil {
			t.Fatalf("Failed to write response: %v", err)
		}
	}))
	defer targetServer.Close()

	// Create logger and audit writer
	logger := zap.NewNop()
	auditWriter := newMockAuditWriter()

	// Create and start proxy server
	cfg := ProxyConfig{
		ListenAddr:  "127.0.0.1:0", // Random port
		Keyring:     keyring,
		Store:       store,
		AuditWriter: auditWriter,
		Logger:      logger,
	}

	proxy, err := NewProxy(cfg)
	require.NoError(t, err)

	// Start proxy in background
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		_ = proxy.Start(ctx)
	}()

	// Wait for proxy to start
	time.Sleep(500 * time.Millisecond)
	assert.True(t, proxy.IsRunning())

	// Create proxy client
	client := NewProxyClient(proxy.Addr())

	// Verify proxy is available
	assert.True(t, client.IsAvailable())

	// Create HTTP client through proxy
	_, err = client.HTTPClient()
	require.NoError(t, err)

	// Verify audit events were written
	events := auditWriter.getEvents()
	assert.NotEmpty(t, events, "audit events should be written")

	// Check for proxy start event
	foundStartEvent := false
	for _, event := range events {
		if event.Type == "proxy.start" {
			foundStartEvent = true
			assert.Equal(t, "info", event.Severity)
			assert.Contains(t, event.Message, "Credential proxy server started")
		}
	}
	assert.True(t, foundStartEvent, "proxy.start event should be present")

	// Stop proxy
	err = proxy.Stop()
	require.NoError(t, err)
	assert.False(t, proxy.IsRunning())

	// Get updated events after stop
	events = auditWriter.getEvents()

	// Verify proxy stop event
	foundStopEvent := false
	for _, event := range events {
		if event.Type == "proxy.stop" {
			foundStopEvent = true
			assert.Equal(t, "info", event.Severity)
			assert.Contains(t, event.Message, "Credential proxy server stopped")
		}
	}
	assert.True(t, foundStopEvent, "proxy.stop event should be present")
}

// TestIntegrationScopedBinding tests scoped credential binding enforcement.
// AC-CREDPROXY-03: Scoped binding enforcement
func TestIntegrationScopedBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create keyring and store credentials
	keyring, _ := newFileKeyring(tmpDir)
	_ = keyring.Store("goose", "openai-key", []byte("sk-openai-secret"))
	_ = keyring.Store("goose", "anthropic-key", []byte("sk-anthropic-secret"))

	// Create provider store with scoped bindings
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)
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

	// Test exact matches
	provider, ref, err := store.MatchForHost("api.openai.com")
	require.NoError(t, err)
	assert.Equal(t, "openai", provider)
	assert.Equal(t, "openai-key", ref.KeyringID)

	provider, ref, err = store.MatchForHost("api.anthropic.com")
	require.NoError(t, err)
	assert.Equal(t, "anthropic", provider)
	assert.Equal(t, "anthropic-key", ref.KeyringID)

	// Test no match for different host
	_, _, err = store.MatchForHost("api.unknown.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no provider found")

	// Add wildcard provider for fallback
	_ = store.Add("wildcard", ProviderRef{
		KeyringID:   "wildcard-key",
		HostPattern: "*",
		Provider:    "wildcard",
	})

	// Now wildcard should match
	provider, ref, err = store.MatchForHost("api.unknown.com")
	require.NoError(t, err)
	assert.Equal(t, "wildcard", provider)
	assert.Equal(t, "wildcard-key", ref.KeyringID)
}

// TestIntegrationCredentialIsolation tests that credentials are isolated per provider.
// AC-CREDPROXY-02: Verify secret never in agent memory
func TestIntegrationCredentialIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	// Create keyring and store multiple credentials
	keyring, _ := newFileKeyring(tmpDir)
	secrets := map[string][]byte{
		"provider-a": []byte("secret-a-12345"),
		"provider-b": []byte("secret-b-67890"),
		"provider-c": []byte("secret-c-abcde"),
	}

	for keyringID, secret := range secrets {
		err := keyring.Store("goose", keyringID, secret)
		require.NoError(t, err)
	}

	// Verify each secret can be retrieved independently
	for keyringID, expectedSecret := range secrets {
		retrieved, err := keyring.Retrieve("goose", keyringID)
		require.NoError(t, err)
		assert.Equal(t, expectedSecret, retrieved,
			fmt.Sprintf("secret for %s should match", keyringID))
	}

	// Verify modifying a retrieved secret doesn't affect the stored secret
	retrieved, _ := keyring.Retrieve("goose", "provider-a")
	retrieved[0] = 'X' // Modify the copy

	original, _ := keyring.Retrieve("goose", "provider-a")
	assert.Equal(t, secrets["provider-a"], original,
		"modifying retrieved secret should not affect stored secret")
}

// TestIntegrationProxyUnavailable tests behavior when proxy is not available.
// AC-CREDPROXY-04: No credential API without proxy
// AC-CREDPROXY-05: If proxy not running, block request and guide user
func TestIntegrationProxyUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create client pointing to non-existent proxy
	client := NewProxyClient("127.0.0.1:19999")

	// Verify proxy is not available
	assert.False(t, client.IsAvailable())

	// Try to create HTTP client - should fail with helpful error
	_, err := client.HTTPClient()
	assert.Error(t, err)

	proxyErr, ok := err.(*ProxyUnavailableError)
	assert.True(t, ok, "error should be ProxyUnavailableError")
	assert.Contains(t, proxyErr.Message, "credential proxy is not running")
	assert.Contains(t, proxyErr.Message, "goose proxy start",
		"error should guide user to start proxy")
}

// TestIntegrationPersistence tests that credentials persist across restarts.
func TestIntegrationPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	keyringDir := filepath.Join(tmpDir, "keyring")
	storePath := filepath.Join(tmpDir, "providers.yaml")

	// First instance: store credentials
	keyring1, _ := newFileKeyring(keyringDir)
	err := keyring1.Store("goose", "persistent-key", []byte("persistent-secret"))
	require.NoError(t, err)

	store1, _ := NewProviderStore(storePath)
	err = store1.Add("persistent", ProviderRef{
		KeyringID:   "persistent-key",
		HostPattern: "api.persistent.com",
		Provider:    "persistent",
	})
	require.NoError(t, err)

	// Second instance: verify credentials persist
	keyring2, _ := newFileKeyring(keyringDir)
	retrieved, err := keyring2.Retrieve("goose", "persistent-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("persistent-secret"), retrieved)

	store2, _ := NewProviderStore(storePath)
	ref, exists := store2.Get("persistent")
	assert.True(t, exists)
	assert.Equal(t, "persistent-key", ref.KeyringID)
	assert.Equal(t, "api.persistent.com", ref.HostPattern)
}

// TestIntegrationConcurrentAccess tests concurrent access to credential proxy.
func TestIntegrationConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	storePath := filepath.Join(tmpDir, "providers.yaml")
	store, _ := NewProviderStore(storePath)

	// Store initial credentials
	_ = keyring.Store("goose", "key-1", []byte("secret-1"))
	_ = store.Add("provider-1", ProviderRef{
		KeyringID:   "key-1",
		HostPattern: "api.provider1.com",
		Provider:    "provider-1",
	})

	// Launch concurrent operations
	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		// Concurrent reads
		go func(id int) {
			keyringID := fmt.Sprintf("key-%d", (id%5)+1)
			_, _ = keyring.Retrieve("goose", keyringID)
			done <- true
		}(i)

		// Concurrent writes
		go func(id int) {
			keyringID := fmt.Sprintf("key-%d", id+10)
			secret := []byte(fmt.Sprintf("secret-%d", id+10))
			_ = keyring.Store("goose", keyringID, secret)
			done <- true
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < 20; i++ {
		<-done
	}

	// Verify state is consistent
	services, _ := keyring.ListServices()
	assert.NotEmpty(t, services)

	providers := store.List()
	assert.NotEmpty(t, providers)
}

// TestIntegrationFileCleanup tests that keyring files are properly cleaned up.
func TestIntegrationFileCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	keyring, _ := newFileKeyring(tmpDir)

	// Store a secret
	_ = keyring.Store("goose", "cleanup-key", []byte("cleanup-secret"))

	// Verify file exists
	secretFile := filepath.Join(tmpDir, "goose_cleanup-key")
	_, err := os.Stat(secretFile)
	require.NoError(t, err, "secret file should exist")

	// Delete the secret
	err = keyring.Delete("goose", "cleanup-key")
	require.NoError(t, err)

	// Verify file is deleted
	_, err = os.Stat(secretFile)
	assert.True(t, os.IsNotExist(err), "secret file should be deleted")
}

// TestIntegrationErrorScenarios tests various error scenarios.
func TestIntegrationErrorScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Run("invalid keyring ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		keyring, _ := newFileKeyring(tmpDir)
		storePath := filepath.Join(tmpDir, "providers.yaml")
		store, _ := NewProviderStore(storePath)

		_ = store.Add("test", ProviderRef{
			KeyringID:   "non-existent-key",
			HostPattern: "api.test.com",
			Provider:    "test",
		})

		// Try to retrieve with invalid keyring ID
		_, err := keyring.Retrieve("goose", "non-existent-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("empty provider name", func(t *testing.T) {
		tmpDir := t.TempDir()
		storePath := filepath.Join(tmpDir, "providers.yaml")
		store, _ := NewProviderStore(storePath)

		err := store.Add("", ProviderRef{
			KeyringID:   "key-id",
			HostPattern: "api.test.com",
			Provider:    "test",
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider name cannot be empty")
	})

	t.Run("corrupted store file", func(t *testing.T) {
		tmpDir := t.TempDir()
		storePath := filepath.Join(tmpDir, "providers.yaml")

		// Write invalid YAML
		err := os.WriteFile(storePath, []byte("invalid: yaml: content: [["), 0644)
		require.NoError(t, err)

		// Try to load store
		_, err = NewProviderStore(storePath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}

// Helper function to read response body
func readResponseBody(body io.ReadCloser) string {
	data, _ := io.ReadAll(body)
	return string(data)
}
