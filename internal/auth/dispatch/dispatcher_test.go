// Package dispatch — unit tests for Dispatcher.
//
// Tests cover:
//   - "keyring" mode: routes to keyring backend exclusively
//   - "file" mode: routes to file backend without calling keyring
//   - "keyring,file" mode: keyring first, fallback to file on ErrKeyringUnavailable
//   - Fallback transition: cache flag prevents redundant keyring probes after
//     the first unavailability
//   - Constructor error path: invalid mode, nil backends
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (SD-1, SD-2, AC-CR-021, AC-CR-022, T-007)
package dispatch

import (
	"errors"
	"sync"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// ---------------------------------------------------------------------------
// Mock credential.Service implementations
// ---------------------------------------------------------------------------

// memBackend is an in-memory credential.Service used for testing.  It records
// calls so tests can assert whether a specific backend was invoked.
type memBackend struct {
	mu          sync.RWMutex
	store       map[string]credential.Credential
	callCount   int
	storeErr    error // injected error returned by Store
	loadErr     error // injected error returned by Load
	unavailable bool  // when true, all calls return ErrKeyringUnavailable
}

func newMemBackend() *memBackend {
	return &memBackend{store: make(map[string]credential.Credential)}
}

// newUnavailableBackend returns a backend that always returns ErrKeyringUnavailable.
func newUnavailableBackend() *memBackend {
	return &memBackend{
		store:       make(map[string]credential.Credential),
		unavailable: true,
	}
}

func (m *memBackend) Store(provider string, cred credential.Credential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.unavailable {
		return credential.ErrKeyringUnavailable
	}
	if m.storeErr != nil {
		return m.storeErr
	}
	m.store[provider] = cred
	return nil
}

func (m *memBackend) Load(provider string) (credential.Credential, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.callCount++
	if m.unavailable {
		return nil, credential.ErrKeyringUnavailable
	}
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	cred, ok := m.store[provider]
	if !ok {
		return nil, credential.ErrNotFound
	}
	return cred, nil
}

func (m *memBackend) Delete(provider string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.unavailable {
		return credential.ErrKeyringUnavailable
	}
	delete(m.store, provider)
	return nil
}

func (m *memBackend) List() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.callCount++
	if m.unavailable {
		return nil, credential.ErrKeyringUnavailable
	}
	ids := make([]string, 0, len(m.store))
	for id := range m.store {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *memBackend) Health(provider string) (credential.HealthStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.callCount++
	if m.unavailable {
		return credential.HealthStatus{}, credential.ErrKeyringUnavailable
	}
	cred, ok := m.store[provider]
	if !ok {
		return credential.HealthStatus{Present: false}, nil
	}
	return credential.HealthStatus{Present: true, MaskedLast4: cred.MaskedString()}, nil
}

// ---------------------------------------------------------------------------
// Constructor tests
// ---------------------------------------------------------------------------

func TestNewDispatcherInvalidMode(t *testing.T) {
	be := newMemBackend()
	_, err := NewDispatcher("invalid_mode", be, be)
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode, got %v", err)
	}
}

func TestNewDispatcherKeyringNilBackend(t *testing.T) {
	_, err := NewDispatcher(ModeKeyring, nil, newMemBackend())
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode for nil keyring, got %v", err)
	}
}

func TestNewDispatcherFileNilBackend(t *testing.T) {
	_, err := NewDispatcher(ModeFile, newMemBackend(), nil)
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode for nil file, got %v", err)
	}
}

func TestNewDispatcherKeyringFileNilKeyring(t *testing.T) {
	_, err := NewDispatcher(ModeKeyringFile, nil, newMemBackend())
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode for nil keyring in keyring,file mode, got %v", err)
	}
}

func TestNewDispatcherKeyringFileNilFile(t *testing.T) {
	_, err := NewDispatcher(ModeKeyringFile, newMemBackend(), nil)
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("expected ErrInvalidMode for nil file in keyring,file mode, got %v", err)
	}
}

func TestNewDispatcherOPPlaceholdersRejected(t *testing.T) {
	be := newMemBackend()
	for _, mode := range []string{"hsm", "op-cli"} {
		_, err := NewDispatcher(mode, be, be)
		if !errors.Is(err, ErrInvalidMode) {
			t.Errorf("mode %q: expected ErrInvalidMode, got %v", mode, err)
		}
	}
}

// ---------------------------------------------------------------------------
// ModeKeyring tests
// ---------------------------------------------------------------------------

func TestKeyringModeRoutesToKeyring(t *testing.T) {
	kr := newMemBackend()
	file := newMemBackend()

	d, err := NewDispatcher(ModeKeyring, kr, file)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	cred := credential.APIKey{Value: "sk-test"}
	if err := d.Store("anthropic", cred); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if kr.callCount != 1 {
		t.Errorf("keyring callCount = %d, want 1", kr.callCount)
	}
	if file.callCount != 0 {
		t.Errorf("file callCount = %d, want 0 (file must not be called in keyring mode)",
			file.callCount)
	}
}

func TestKeyringModeReturnsErrKeyringUnavailable(t *testing.T) {
	kr := newUnavailableBackend()
	d, err := NewDispatcher(ModeKeyring, kr, nil)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	err = d.Store("anthropic", credential.APIKey{Value: "sk-test"})
	if !errors.Is(err, credential.ErrKeyringUnavailable) {
		t.Errorf("expected ErrKeyringUnavailable, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ModeFile tests
// ---------------------------------------------------------------------------

func TestFileModeDoesNotCallKeyring(t *testing.T) {
	kr := newMemBackend()
	file := newMemBackend()

	d, err := NewDispatcher(ModeFile, kr, file)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	if err := d.Store("anthropic", credential.APIKey{Value: "sk-test"}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if kr.callCount != 0 {
		t.Errorf("keyring callCount = %d, want 0 (keyring must never be called in file mode)",
			kr.callCount)
	}
	if file.callCount != 1 {
		t.Errorf("file callCount = %d, want 1", file.callCount)
	}
}

// ---------------------------------------------------------------------------
// ModeKeyringFile fallback tests
// ---------------------------------------------------------------------------

func TestKeyringFileModeSuccessUsesKeyring(t *testing.T) {
	kr := newMemBackend()
	file := newMemBackend()

	d, err := NewDispatcher(ModeKeyringFile, kr, file)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	if err := d.Store("anthropic", credential.APIKey{Value: "sk-test"}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	if kr.callCount != 1 {
		t.Errorf("keyring callCount = %d, want 1", kr.callCount)
	}
	if file.callCount != 0 {
		t.Errorf("file callCount = %d, want 0", file.callCount)
	}
}

func TestKeyringFileModeUnavailableFallsBackToFile(t *testing.T) {
	kr := newUnavailableBackend()
	file := newMemBackend()

	d, err := NewDispatcher(ModeKeyringFile, kr, file)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	// First call: keyring returns ErrKeyringUnavailable → fallback to file.
	if err := d.Store("anthropic", credential.APIKey{Value: "sk-test"}); err != nil {
		t.Fatalf("Store (fallback): %v", err)
	}

	if kr.callCount != 1 {
		t.Errorf("after first call: keyring callCount = %d, want 1", kr.callCount)
	}
	if file.callCount != 1 {
		t.Errorf("after first call: file callCount = %d, want 1", file.callCount)
	}

	// Second call: unavailability cached → keyring not called again.
	_ = d.Store("deepseek", credential.APIKey{Value: "ds-key"})

	if kr.callCount != 1 {
		t.Errorf("after second call: keyring callCount = %d, want 1 (short-circuit active)",
			kr.callCount)
	}
	if file.callCount != 2 {
		t.Errorf("after second call: file callCount = %d, want 2", file.callCount)
	}
}

// TestKeyringFileModeUnavailabilityTransition verifies the full fallback
// transition: once keyring is marked unavailable, Load also goes to file.
func TestKeyringFileModeUnavailabilityTransition(t *testing.T) {
	kr := newUnavailableBackend()
	file := newMemBackend()

	d, err := NewDispatcher(ModeKeyringFile, kr, file)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	// Store triggers fallback — credential lands in file.
	cred := credential.APIKey{Value: "sk-fallback-test"}
	if err := d.Store("anthropic", cred); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Load should now also route to file (cached unavailability).
	loaded, err := d.Load("anthropic")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	apiKey, ok := loaded.(credential.APIKey)
	if !ok || apiKey.Value != cred.Value {
		t.Errorf("Load: got %v, want %v", loaded, cred)
	}

	// Keyring was called once (for Store), never again (for Load).
	if kr.callCount != 1 {
		t.Errorf("keyring callCount = %d, want 1", kr.callCount)
	}
}

// TestKeyringFileModeNonUnavailableError verifies that errors other than
// ErrKeyringUnavailable are not swallowed and do NOT trigger fallback.
func TestKeyringFileModeNonUnavailableError(t *testing.T) {
	kr := newMemBackend()
	kr.storeErr = errors.New("some other keyring error")
	file := newMemBackend()

	d, err := NewDispatcher(ModeKeyringFile, kr, file)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	err = d.Store("anthropic", credential.APIKey{Value: "sk-test"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if errors.Is(err, credential.ErrKeyringUnavailable) {
		t.Error("unexpected ErrKeyringUnavailable — the error should propagate as-is")
	}

	// File backend must not have been called.
	if file.callCount != 0 {
		t.Errorf("file callCount = %d, want 0", file.callCount)
	}
}
