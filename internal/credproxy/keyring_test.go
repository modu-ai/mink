package credproxy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileKeyringStore tests storing secrets in the file keyring.
func TestFileKeyringStore(t *testing.T) {
	t.Parallel()

	// Create temporary directory for testing
	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Store a secret
	secret := []byte("test-api-key-12345")
	err = keyring.Store("test-service", "test-key", secret)
	require.NoError(t, err)

	// Verify secret was stored
	retrieved, err := keyring.Retrieve("test-service", "test-key")
	require.NoError(t, err)
	assert.Equal(t, secret, retrieved)

	// Verify secret file exists on disk
	secretFile := filepath.Join(tmpDir, "test-service_test-key")
	_, err = os.Stat(secretFile)
	require.NoError(t, err)
}

// TestFileKeyringRetrieve tests retrieving secrets from the file keyring.
func TestFileKeyringRetrieve(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Store a secret
	secret := []byte("another-secret-key")
	err = keyring.Store("service-a", "key-a", secret)
	require.NoError(t, err)

	// Retrieve the secret
	retrieved, err := keyring.Retrieve("service-a", "key-a")
	require.NoError(t, err)
	assert.Equal(t, secret, retrieved)
}

// TestFileKeyringRetrieveNotFound tests retrieving a non-existent secret.
func TestFileKeyringRetrieveNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Try to retrieve non-existent service
	_, err = keyring.Retrieve("non-existent", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service not found")

	// Create a service and try to retrieve non-existent key
	_ = keyring.Store("service", "key", []byte("secret"))
	_, err = keyring.Retrieve("service", "non-existent-key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

// TestFileKeyringDelete tests deleting secrets from the file keyring.
func TestFileKeyringDelete(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Store a secret
	secret := []byte("deletable-secret")
	err = keyring.Store("service", "key", secret)
	require.NoError(t, err)

	// Delete the secret
	err = keyring.Delete("service", "key")
	require.NoError(t, err)

	// Verify secret is deleted
	_, err = keyring.Retrieve("service", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

// TestFileKeyringDeleteNotFound tests deleting a non-existent secret.
func TestFileKeyringDeleteNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Try to delete non-existent service
	err = keyring.Delete("non-existent", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service not found")
}

// TestFileKeyringListServices tests listing services in the keyring.
func TestFileKeyringListServices(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Initially empty
	services, err := keyring.ListServices()
	require.NoError(t, err)
	assert.Empty(t, services)

	// Add services
	_ = keyring.Store("service-1", "key", []byte("secret-1"))
	_ = keyring.Store("service-2", "key", []byte("secret-2"))
	_ = keyring.Store("service-3", "key", []byte("secret-3"))

	// List services
	services, err = keyring.ListServices()
	require.NoError(t, err)
	assert.Len(t, services, 3)
	assert.Contains(t, services, "service-1")
	assert.Contains(t, services, "service-2")
	assert.Contains(t, services, "service-3")
}

// TestFileKeyringPersistence tests that secrets persist across keyring instances.
func TestFileKeyringPersistence(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create first keyring instance and store a secret
	keyring1, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	secret := []byte("persistent-secret")
	err = keyring1.Store("service", "key", secret)
	require.NoError(t, err)

	// Create second keyring instance and verify secret is loaded
	keyring2, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	retrieved, err := keyring2.Retrieve("service", "key")
	require.NoError(t, err)
	assert.Equal(t, secret, retrieved)
}

// TestFileKeyringValidation tests input validation.
func TestFileKeyringValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Test empty service name
	err = keyring.Store("", "key", []byte("secret"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service name cannot be empty")

	// Test empty key name
	err = keyring.Store("service", "", []byte("secret"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key name cannot be empty")

	// Test retrieve with empty service name
	_, err = keyring.Retrieve("", "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service name cannot be empty")

	// Test retrieve with empty key name
	_, err = keyring.Retrieve("service", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key name cannot be empty")
}

// TestFileKeyringMemorySafety tests that retrieved secrets are copies.
func TestFileKeyringMemorySafety(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	// Store a secret
	secret := []byte("original-secret")
	err = keyring.Store("service", "key", secret)
	require.NoError(t, err)

	// Retrieve the secret
	retrieved, err := keyring.Retrieve("service", "key")
	require.NoError(t, err)

	// Modify the retrieved secret
	retrieved[0] = 'X'

	// Verify the original secret is unchanged
	original, err := keyring.Retrieve("service", "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("original-secret"), original)
	assert.NotEqual(t, retrieved, original)
}

// TestFileKeyringLoadFromDisk tests loading existing secrets from disk.
func TestFileKeyringLoadFromDisk(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Manually create a secret file
	secretFile := filepath.Join(tmpDir, "manual-service_manual-key")
	err := os.WriteFile(secretFile, []byte("manual-secret"), 0600)
	require.NoError(t, err)

	// Create keyring and verify it loads the secret
	keyring, err := newFileKeyring(tmpDir)
	require.NoError(t, err)

	retrieved, err := keyring.Retrieve("manual-service", "manual-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("manual-secret"), retrieved)
}

// BenchmarkFileKeyringStore benchmarks storing secrets.
func BenchmarkFileKeyringStore(b *testing.B) {
	tmpDir := b.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	secret := []byte("benchmark-secret-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service := fmt.Sprintf("service-%d", i%100)
		key := fmt.Sprintf("key-%d", i%1000)
		_ = keyring.Store(service, key, secret)
	}
}

// BenchmarkFileKeyringRetrieve benchmarks retrieving secrets.
func BenchmarkFileKeyringRetrieve(b *testing.B) {
	tmpDir := b.TempDir()
	keyring, _ := newFileKeyring(tmpDir)
	secret := []byte("benchmark-secret-key")

	// Pre-populate keyring
	for i := 0; i < 100; i++ {
		service := fmt.Sprintf("service-%d", i)
		key := fmt.Sprintf("key-%d", i)
		_ = keyring.Store(service, key, secret)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service := fmt.Sprintf("service-%d", i%100)
		key := fmt.Sprintf("key-%d", i%100)
		_, _ = keyring.Retrieve(service, key)
	}
}
