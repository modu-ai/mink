// Package onboarding — keyring.go provides the OS keyring abstraction used by the
// onboarding flow to persist provider API keys outside of disk-resident config.yaml.
//
// Architecture overview:
//
//   - KeyringClient is the abstract contract for all secret-bearing paths.
//   - SystemKeyring wraps github.com/zalando/go-keyring against the host OS backend
//     (macOS Keychain Services, Linux Secret Service via libsecret, Windows Credential
//     Manager).
//   - InMemoryKeyring is a process-local substitute for CI headless environments
//     (no dbus daemon, no Keychain signing identity) and unit tests.
//
// Key naming convention:
//
//	service = "mink"  (constant)
//	user    = "provider.<name>.api_key"
//
// zalando/go-keyring uses (service, user, password) as its three parameters.
// All onboarding entries store secrets under service="mink" with the structured
// provider key string in the user field.
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6.6
// REQ: REQ-OB-007
package onboarding

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	zkeyring "github.com/zalando/go-keyring"
)

// keyringService is the service name used for all keyring entries.
// It must not change once entries are written to the host OS backend.
// Migration to a different service name requires an explicit migration SPEC.
// @MX:NOTE: [AUTO] Changing this constant silently orphans all existing keyring
// entries; a migration path must be specified before renaming.
const keyringService = "mink"

// providerKeyPrefix and apiKeySuffix are the components of the structured key.
const (
	providerKeyPrefix = "provider"
	apiKeySuffix      = "api_key"
)

// Sentinel errors for KeyringClient operations.
// Callers use errors.Is to distinguish error categories.
var (
	// ErrKeyNotFound is returned by Get and Delete when the requested entry
	// is not present in the keyring. It wraps zalando's keyring.ErrNotFound
	// for SystemKeyring and is returned directly by InMemoryKeyring.
	ErrKeyNotFound = errors.New("keyring entry not found")

	// ErrNilKeyringClient is returned by the high-level provider helpers when
	// the supplied KeyringClient argument is nil.
	ErrNilKeyringClient = errors.New("keyring client is nil")

	// ErrInvalidProviderName is returned when the provider name normalises to
	// an empty string (blank or whitespace-only input).
	ErrInvalidProviderName = errors.New("provider name is empty")

	// ErrKeyringEmptyAPIKey is returned by SetProviderAPIKey when the supplied
	// API key string is empty. This is distinct from validators.go's ErrEmptyAPIKey
	// (which validates format during user input) to avoid a sentinel collision
	// between the two packages within the same internal package scope.
	ErrKeyringEmptyAPIKey = errors.New("API key must not be empty")
)

// KeyringClient is the abstract OS keyring contract used by the onboarding
// flow to persist provider API keys outside of disk-resident config.yaml.
// The interface decouples production code from the underlying OS backend so
// that tests can substitute an in-memory implementation without requiring a
// dbus daemon or Keychain signing identity.
//
// All keys passed to Set/Get/Delete are opaque strings; callers should use
// the providerEntryKey helper to construct correctly namespaced keys.
//
// @MX:ANCHOR: [AUTO] KeyringClient is the integration boundary for all
// secret-bearing paths in onboarding (Phase 1F SubmitStep step 5, Phase 1E
// Completion, future re-config flow).
// @MX:REASON: Backend swap (SystemKeyring vs InMemoryKeyring) must be
// transparent. ErrKeyNotFound semantic stability is part of the contract;
// callers check errors.Is(err, ErrKeyNotFound) for absent-entry logic.
// @MX:SPEC: SPEC-MINK-ONBOARDING-001 REQ-OB-007 §6.6
type KeyringClient interface {
	// Set stores or replaces the secret for the given key under service "mink".
	Set(key string, secret string) error

	// Get retrieves the secret for the given key.
	// Returns ErrKeyNotFound when no entry exists for that key.
	Get(key string) (string, error)

	// Delete removes the entry for the given key.
	// Returns nil even when the entry was absent (idempotent).
	Delete(key string) error
}

// SystemKeyring wraps github.com/zalando/go-keyring against the host OS backend.
// The zero value is usable; no constructor call is required.
//
// @MX:NOTE: [AUTO] CI headless caveat: ubuntu runners lack a functioning dbus /
// Secret Service, and macOS runners without a valid signing identity reject
// Keychain writes. Tests and CI pipelines MUST use InMemoryKeyring instead.
// Production code that runs on an installed host OS may safely use SystemKeyring.
type SystemKeyring struct{}

// Set stores or replaces the secret for the given key in the OS keyring.
func (SystemKeyring) Set(key string, secret string) error {
	if err := zkeyring.Set(keyringService, key, secret); err != nil {
		return fmt.Errorf("keyring set %q: %w", key, err)
	}
	return nil
}

// Get retrieves the secret for the given key from the OS keyring.
// Returns ErrKeyNotFound when no entry exists.
func (SystemKeyring) Get(key string) (string, error) {
	val, err := zkeyring.Get(keyringService, key)
	if err != nil {
		if errors.Is(err, zkeyring.ErrNotFound) {
			return "", ErrKeyNotFound
		}
		return "", fmt.Errorf("keyring get %q: %w", key, err)
	}
	return val, nil
}

// Delete removes the entry for the given key from the OS keyring.
// Returns nil when the entry was already absent (idempotent).
func (SystemKeyring) Delete(key string) error {
	err := zkeyring.Delete(keyringService, key)
	if err != nil {
		if errors.Is(err, zkeyring.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("keyring delete %q: %w", key, err)
	}
	return nil
}

// InMemoryKeyring is a process-local KeyringClient backed by a plain map.
// It is safe for concurrent use via an internal RWMutex.
//
// Use cases:
//   - Unit and integration tests that run without a dbus daemon or Keychain.
//   - CI headless runners (ubuntu, macOS in sandbox) where OS keyring is unavailable.
//   - Dry-run / preview modes where writes must not touch the host OS.
//
// InMemoryKeyring does NOT persist entries across process restarts.
type InMemoryKeyring struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewInMemoryKeyring returns an empty in-memory keyring ready for use.
func NewInMemoryKeyring() *InMemoryKeyring {
	return &InMemoryKeyring{
		store: make(map[string]string),
	}
}

// Set stores or overwrites the secret for the given key.
func (m *InMemoryKeyring) Set(key string, secret string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = secret
	return nil
}

// Get retrieves the secret for the given key.
// Returns ErrKeyNotFound when no entry exists for that key.
func (m *InMemoryKeyring) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.store[key]
	if !ok {
		return "", ErrKeyNotFound
	}
	return val, nil
}

// Delete removes the entry for the given key. Returns nil when the entry
// was absent (idempotent).
func (m *InMemoryKeyring) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.store, key)
	return nil
}

// providerEntryKey builds the canonical entry key "provider.<name>.api_key".
// name is normalised via strings.ToLower + strings.TrimSpace.
// Returns ErrInvalidProviderName when the normalised name is empty.
func providerEntryKey(name string) (string, error) {
	normalised := strings.ToLower(strings.TrimSpace(name))
	if normalised == "" {
		return "", ErrInvalidProviderName
	}
	return fmt.Sprintf("%s.%s.%s", providerKeyPrefix, normalised, apiKeySuffix), nil
}

// SetProviderAPIKey stores the API key for the given provider via the supplied
// KeyringClient under the canonical entry key "provider.<name>.api_key".
//
// Error conditions:
//   - c == nil → ErrNilKeyringClient
//   - provider normalises to "" → ErrInvalidProviderName
//   - key == "" → ErrKeyringEmptyAPIKey (defense-in-depth; callers should
//     pre-validate with ValidateProviderAPIKey from validators.go)
func SetProviderAPIKey(c KeyringClient, provider string, key string) error {
	if c == nil {
		return ErrNilKeyringClient
	}
	if key == "" {
		return ErrKeyringEmptyAPIKey
	}
	entry, err := providerEntryKey(provider)
	if err != nil {
		return err
	}
	return c.Set(entry, key)
}

// GetProviderAPIKey reads the API key for the given provider via the supplied
// KeyringClient.
//
// Error conditions:
//   - c == nil → ErrNilKeyringClient
//   - provider normalises to "" → ErrInvalidProviderName
//   - entry absent → ErrKeyNotFound (propagated from KeyringClient.Get)
func GetProviderAPIKey(c KeyringClient, provider string) (string, error) {
	if c == nil {
		return "", ErrNilKeyringClient
	}
	entry, err := providerEntryKey(provider)
	if err != nil {
		return "", err
	}
	return c.Get(entry)
}

// DeleteProviderAPIKey removes the API key entry for the given provider.
// Returns nil even when the entry was absent (idempotent best-effort).
//
// Error conditions:
//   - c == nil → ErrNilKeyringClient
//   - provider normalises to "" → ErrInvalidProviderName
func DeleteProviderAPIKey(c KeyringClient, provider string) error {
	if c == nil {
		return ErrNilKeyringClient
	}
	entry, err := providerEntryKey(provider)
	if err != nil {
		return err
	}
	return c.Delete(entry)
}
