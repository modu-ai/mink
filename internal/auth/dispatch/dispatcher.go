// Package dispatch provides the config-driven credential backend router.
//
// The Dispatcher implements credential.Service and routes each call to the
// appropriate backend based on the auth.store configuration value:
//
//   - "keyring"      – keyring backend only; ErrKeyringUnavailable is returned
//     to the caller without falling back.
//   - "file"         – file backend only; the keyring API is never invoked.
//   - "keyring,file" – keyring first; on ErrKeyringUnavailable the call is
//     transparently routed to the file backend.  The unavailability is cached
//     via a simple boolean flag (no TTL / recovery probe — see @MX:NOTE below)
//     to avoid redundant probing within the same process session.
//
// Any other auth.store value causes NewDispatcher to return an error.
//
// Both backends must implement credential.Service.  NewDispatcher enforces
// the constraint that both backends are non-nil for "keyring,file" mode, and
// that the relevant backend is non-nil for single-backend modes.
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (SD-1, SD-2, AC-CR-021, AC-CR-022, T-007)
package dispatch

import (
	"errors"
	"fmt"
	"sync"

	"github.com/modu-ai/mink/internal/auth/credential"
)

// Mode constants define the valid auth.store configuration values.
const (
	// ModeKeyring routes all calls to the keyring backend.
	ModeKeyring = "keyring"

	// ModeFile routes all calls to the file backend without touching keyring.
	ModeFile = "file"

	// ModeKeyringFile tries the keyring first and falls back to file on
	// ErrKeyringUnavailable.
	ModeKeyringFile = "keyring,file"
)

// ErrInvalidMode is returned by NewDispatcher when mode is not one of the
// three recognised values.
var ErrInvalidMode = errors.New("auth.store: invalid mode")

// Dispatcher implements credential.Service with config-driven backend
// selection.  It is safe for concurrent use.
//
// @MX:ANCHOR: [AUTO] Dispatcher is the primary routing layer consumed by the
// CLI, LLM-ROUTING-V2, and integration tests (fan_in >= 3).
// @MX:REASON: Any change to dispatch logic or constructor signature propagates
// to every consumer that resolves credentials at runtime.
// @MX:SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (SD-1, SD-2, T-007)
type Dispatcher struct {
	mode      string
	keyringBE credential.Service
	fileBE    credential.Service

	// keyringUnavailable is set to true the first time a call returns
	// ErrKeyringUnavailable in "keyring,file" mode.  Subsequent calls skip the
	// keyring probe entirely.
	//
	// @MX:NOTE: [AUTO] Simple boolean cache — no TTL or recovery probe
	// implemented (intentional; SPEC M2 constraint).  If the keyring becomes
	// available again after the process starts, the fallback remains active
	// until the process restarts.  This is a known limitation documented in
	// plan.md §5 risk R5 and the Dispatcher struct comment.
	keyringUnavailable bool
	mu                 sync.RWMutex
}

// NewDispatcher creates a Dispatcher for the given mode.
//
// Constraints:
//   - "keyring": keyringBE must be non-nil; fileBE may be nil.
//   - "file":    fileBE must be non-nil; keyringBE may be nil.
//   - "keyring,file": both keyringBE and fileBE must be non-nil.
//   - Any other value: returns ErrInvalidMode.
func NewDispatcher(mode string, keyringBE, fileBE credential.Service) (*Dispatcher, error) {
	switch mode {
	case ModeKeyring:
		if keyringBE == nil {
			return nil, fmt.Errorf("dispatch: %w: keyring backend is nil for mode %q",
				ErrInvalidMode, mode)
		}
	case ModeFile:
		if fileBE == nil {
			return nil, fmt.Errorf("dispatch: %w: file backend is nil for mode %q",
				ErrInvalidMode, mode)
		}
	case ModeKeyringFile:
		if keyringBE == nil || fileBE == nil {
			return nil, fmt.Errorf("dispatch: %w: both backends must be non-nil for mode %q",
				ErrInvalidMode, mode)
		}
	default:
		return nil, fmt.Errorf("dispatch: %w: %q is not one of keyring, file, keyring,file",
			ErrInvalidMode, mode)
	}

	return &Dispatcher{
		mode:      mode,
		keyringBE: keyringBE,
		fileBE:    fileBE,
	}, nil
}

// ---------------------------------------------------------------------------
// credential.Service implementation
// ---------------------------------------------------------------------------

// Store persists cred for provider using the active backend per the configured
// mode.  In "keyring,file" mode, a keyring unavailability on Store causes the
// unavailability to be cached and the write to be redirected to the file
// backend.
func (d *Dispatcher) Store(provider string, cred credential.Credential) error {
	return d.dispatch(func(be credential.Service) error {
		return be.Store(provider, cred)
	})
}

// Load retrieves the credential for provider from the active backend.
func (d *Dispatcher) Load(provider string) (credential.Credential, error) {
	var result credential.Credential
	err := d.dispatch(func(be credential.Service) error {
		var loadErr error
		result, loadErr = be.Load(provider)
		return loadErr
	})
	return result, err
}

// Delete removes the credential for provider from the active backend.
func (d *Dispatcher) Delete(provider string) error {
	return d.dispatch(func(be credential.Service) error {
		return be.Delete(provider)
	})
}

// List returns provider IDs that have a stored credential.
func (d *Dispatcher) List() ([]string, error) {
	var result []string
	err := d.dispatch(func(be credential.Service) error {
		var listErr error
		result, listErr = be.List()
		return listErr
	})
	return result, err
}

// Health probes the presence and masked value for provider.
func (d *Dispatcher) Health(provider string) (credential.HealthStatus, error) {
	var result credential.HealthStatus
	err := d.dispatch(func(be credential.Service) error {
		var healthErr error
		result, healthErr = be.Health(provider)
		return healthErr
	})
	return result, err
}

// ---------------------------------------------------------------------------
// Internal dispatch logic
// ---------------------------------------------------------------------------

// dispatch executes op against the backend selected by the configured mode.
//
// For "keyring,file" mode it applies the fallback transition:
//  1. If the cached unavailability flag is already set, route directly to file.
//  2. Otherwise, try keyring.  If keyring returns ErrKeyringUnavailable, mark
//     the cache flag, then retry op against the file backend.
func (d *Dispatcher) dispatch(op func(credential.Service) error) error {
	switch d.mode {
	case ModeKeyring:
		return op(d.keyringBE)

	case ModeFile:
		return op(d.fileBE)

	case ModeKeyringFile:
		return d.dispatchWithFallback(op)

	default:
		// Should be unreachable because NewDispatcher validates the mode.
		return fmt.Errorf("dispatch: internal error: unknown mode %q", d.mode)
	}
}

// dispatchWithFallback implements the "keyring,file" auto-fallback logic.
func (d *Dispatcher) dispatchWithFallback(op func(credential.Service) error) error {
	// Fast path: if we already know keyring is unavailable in this session,
	// route directly to the file backend.
	d.mu.RLock()
	unavailable := d.keyringUnavailable
	d.mu.RUnlock()

	if unavailable {
		return op(d.fileBE)
	}

	// Try the keyring backend first.
	err := op(d.keyringBE)
	if err == nil {
		return nil
	}

	// On ErrKeyringUnavailable, cache the flag and fall back to file.
	if errors.Is(err, credential.ErrKeyringUnavailable) {
		d.mu.Lock()
		d.keyringUnavailable = true
		d.mu.Unlock()

		return op(d.fileBE)
	}

	// Any other error is returned as-is.
	return err
}
