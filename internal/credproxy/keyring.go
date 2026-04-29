// Package credproxy provides zero-knowledge credential proxy for the Goose agent runtime.
// SPEC-GOOSE-CREDENTIAL-PROXY-001
//
// The credential proxy ensures that secret values never enter agent process memory
// by mediating access through a local proxy server that retrieves credentials from
// the OS keyring and injects them into outbound API requests.
package credproxy

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// KeyringBackend abstracts OS-specific credential storage.
// REQ-CREDPROXY-001: Store secret in OS keyring, save reference only
//
// @MX:ANCHOR: [AUTO] Keyring storage interface
// @MX:REASON: Multiple implementations (file, native, mock) depend on this interface, fan_in >= 3
// @MX:SPEC: SPEC-GOOSE-CREDENTIAL-PROXY-001 REQ-CREDPROXY-001, AC-CREDPROXY-01
type KeyringBackend interface {
	// Store stores a secret in the keyring.
	Store(service, key string, secret []byte) error

	// Retrieve retrieves a secret from the keyring.
	Retrieve(service, key string) ([]byte, error)

	// Delete removes a secret from the keyring.
	Delete(service, key string) error

	// ListServices returns all services that have credentials stored.
	ListServices() ([]string, error)
}

// fileKeyring is a file-based mock backend for testing.
// In production, this would be replaced with native OS keyring integration.
//
// @MX:NOTE: [AUTO] File-based keyring for testing without CGO dependencies
// @MX:TEST: Use mockFileKeyring in tests to avoid OS keyring dependencies
type fileKeyring struct {
	mu   sync.RWMutex
	dir  string
	secrets map[string]map[string][]byte // service -> key -> secret
}

// newFileKeyring creates a new file-based keyring backend.
// The secrets are stored in individual files under the specified directory.
//
// REQ-CREDPROXY-001: References stored in ~/.goose/secrets/providers.yaml
// The actual secret values are stored separately in the keyring backend.
func newFileKeyring(dir string) (*fileKeyring, error) {
	// SECURITY: Set umask to 0 before creating secret directory to ensure
	// exact 0700 permissions regardless of process umask.
	oldUmask := syscall.Umask(0)
	defer syscall.Umask(oldUmask)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keyring directory: %w", err)
	}

	fk := &fileKeyring{
		dir:  dir,
		secrets: make(map[string]map[string][]byte),
	}

	// Load existing secrets from disk
	if err := fk.loadFromDisk(); err != nil {
		return nil, fmt.Errorf("failed to load existing secrets: %w", err)
	}

	return fk, nil
}

// Store stores a secret in the file-based keyring.
func (fk *fileKeyring) Store(service, key string, secret []byte) error {
	if service == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if key == "" {
		return fmt.Errorf("key name cannot be empty")
	}

	fk.mu.Lock()
	defer fk.mu.Unlock()

	// Initialize service map if needed
	if fk.secrets[service] == nil {
		fk.secrets[service] = make(map[string][]byte)
	}

	// Store secret in memory
	fk.secrets[service][key] = secret

	// Persist to disk
	if err := fk.saveToDisk(service, key, secret); err != nil {
		return fmt.Errorf("failed to persist secret: %w", err)
	}

	return nil
}

// Retrieve retrieves a secret from the file-based keyring.
func (fk *fileKeyring) Retrieve(service, key string) ([]byte, error) {
	if service == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}
	if key == "" {
		return nil, fmt.Errorf("key name cannot be empty")
	}

	fk.mu.RLock()
	defer fk.mu.RUnlock()

	// Check if service exists
	if fk.secrets[service] == nil {
		return nil, fmt.Errorf("service not found: %s", service)
	}

	// Check if key exists
	secret, exists := fk.secrets[service][key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s/%s", service, key)
	}

	// Return a copy to prevent external modification
	result := make([]byte, len(secret))
	copy(result, secret)
	return result, nil
}

// Delete removes a secret from the file-based keyring.
func (fk *fileKeyring) Delete(service, key string) error {
	if service == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if key == "" {
		return fmt.Errorf("key name cannot be empty")
	}

	fk.mu.Lock()
	defer fk.mu.Unlock()

	// Check if service exists
	if fk.secrets[service] == nil {
		return fmt.Errorf("service not found: %s", service)
	}

	// Check if key exists
	if _, exists := fk.secrets[service][key]; !exists {
		return fmt.Errorf("key not found: %s/%s", service, key)
	}

	// Delete from memory
	delete(fk.secrets[service], key)

	// Delete from disk
	filePath := fk.getFilePath(service, key)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete secret file: %w", err)
	}

	return nil
}

// ListServices returns all services that have credentials stored.
func (fk *fileKeyring) ListServices() ([]string, error) {
	fk.mu.RLock()
	defer fk.mu.RUnlock()

	services := make([]string, 0, len(fk.secrets))
	for service := range fk.secrets {
		services = append(services, service)
	}
	return services, nil
}

// loadFromDisk loads existing secrets from disk into memory.
func (fk *fileKeyring) loadFromDisk() error {
	entries, err := os.ReadDir(fk.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist yet, which is fine
		}
		return fmt.Errorf("failed to read keyring directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories
		}

		// Parse filename format: service_key
		name := entry.Name()
		service, key, err := parseFilename(name)
		if err != nil {
			continue // Skip invalid filenames
		}

		// Read secret file
		filePath := filepath.Join(fk.dir, name)
		secret, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read secret file %s: %w", filePath, err)
		}

		// Store in memory
		if fk.secrets[service] == nil {
			fk.secrets[service] = make(map[string][]byte)
		}
		fk.secrets[service][key] = secret
	}

	return nil
}

// saveToDisk saves a secret to disk.
func (fk *fileKeyring) saveToDisk(service, key string, secret []byte) error {
	filePath := fk.getFilePath(service, key)

	// SECURITY: Set umask to 0 before writing secret file to ensure
	// exact 0600 permissions regardless of process umask.
	oldUmask := syscall.Umask(0)
	defer syscall.Umask(oldUmask)

	// Write secret file with restrictive permissions
	if err := os.WriteFile(filePath, secret, 0600); err != nil {
		return fmt.Errorf("failed to write secret file: %w", err)
	}

	return nil
}

// getFilePath returns the file path for a given service and key.
func (fk *fileKeyring) getFilePath(service, key string) string {
	// Use format: service_key for filenames
	filename := fmt.Sprintf("%s_%s", service, key)
	return filepath.Join(fk.dir, filename)
}

// parseFilename parses a filename back into service and key components.
func parseFilename(filename string) (service, key string, err error) {
	// Find the last underscore that separates service from key
	// This is a simple implementation; production should use more robust parsing
	lastUnderscore := -1
	for i := len(filename) - 1; i >= 0; i-- {
		if filename[i] == '_' {
			lastUnderscore = i
			break
		}
	}

	if lastUnderscore == -1 || lastUnderscore == 0 || lastUnderscore == len(filename)-1 {
		return "", "", fmt.Errorf("invalid filename format")
	}

	service = filename[:lastUnderscore]
	key = filename[lastUnderscore+1:]
	return service, key, nil
}
