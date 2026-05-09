package telegram

import (
	"fmt"
	"sync"
)

// Keyring defines the narrow interface for storing and retrieving secrets.
// The production implementation wraps credproxy (wired in P2).
// The in-memory fallback (MemoryKeyring) is used in tests and when credproxy
// is unavailable.
//
// @MX:ANCHOR: [AUTO] Keyring abstracts secret storage for the bot token.
// @MX:REASON: SPEC-GOOSE-MSG-TELEGRAM-001 U02/N01; fan_in via setup command, bootstrap, and tests.
type Keyring interface {
	// Store saves a secret value under service+key.
	Store(service, key string, value []byte) error

	// Retrieve fetches a secret value by service+key.
	// Returns an error if the secret is not found.
	Retrieve(service, key string) ([]byte, error)
}

// KeyringService and KeyringKey are the constants used to store the bot token.
const (
	KeyringService = "goose-messaging"
	KeyringKey     = "telegram.bot.token"
)

// MemoryKeyring is an in-memory Keyring implementation used in tests
// and as a graceful fallback when credproxy is not available.
type MemoryKeyring struct {
	mu    sync.RWMutex
	store map[string][]byte
}

// NewMemoryKeyring creates a new empty MemoryKeyring.
func NewMemoryKeyring() *MemoryKeyring {
	return &MemoryKeyring{store: make(map[string][]byte)}
}

// Store saves a value under "service:key".
func (m *MemoryKeyring) Store(service, key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := service + ":" + key
	cp := make([]byte, len(value))
	copy(cp, value)
	m.store[k] = cp
	return nil
}

// Retrieve fetches a value by "service:key".
func (m *MemoryKeyring) Retrieve(service, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k := service + ":" + key
	v, ok := m.store[k]
	if !ok {
		return nil, fmt.Errorf("keyring: secret not found: %s/%s", service, key)
	}
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, nil
}

// @MX:TODO P2 — wire CredproxyKeyring to credproxy.NewProxy when public
// constructor is available in SPEC-GOOSE-CREDENTIAL-PROXY-001.
