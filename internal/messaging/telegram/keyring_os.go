//go:build !nokeyring

package telegram

import (
	"fmt"

	zk "github.com/zalando/go-keyring"
)

// OSKeyring is the production Keyring implementation backed by the OS secret store.
// On macOS it uses Keychain, on Windows Credential Manager, and on Linux the
// D-Bus Secret Service (requires the dbus daemon and a running secret service).
//
// Build tag !nokeyring ensures this file is excluded from CI builds and
// -tags=nokeyring test runs where dbus is unavailable (strategy-p3.md §E.4).
type OSKeyring struct{}

// NewOSKeyring constructs an OSKeyring.
func NewOSKeyring() *OSKeyring { return &OSKeyring{} }

// Store saves value under service+key in the OS keyring.
func (OSKeyring) Store(service, key string, value []byte) error {
	if err := zk.Set(service, key, string(value)); err != nil {
		return fmt.Errorf("OSKeyring.Store: %w", err)
	}
	return nil
}

// Retrieve fetches the secret stored under service+key.
// Returns an error wrapping keyring.ErrNotFound when the secret does not exist.
func (OSKeyring) Retrieve(service, key string) ([]byte, error) {
	s, err := zk.Get(service, key)
	if err != nil {
		return nil, fmt.Errorf("OSKeyring.Retrieve: %w", err)
	}
	return []byte(s), nil
}
