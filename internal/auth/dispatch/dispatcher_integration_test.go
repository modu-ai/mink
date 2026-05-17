//go:build integration

// Package dispatch — integration test for the full fallback path.
//
// This test wires a real file.Backend (in a temp dir) and a controlled wrapper
// around keyring.Backend that forces ErrKeyringUnavailable on demand.  It
// exercises the Dispatcher in "keyring,file" mode and verifies the
// end-to-end fallback transition.
//
// Run with:
//
//	go test -tags=integration -race ./internal/auth/dispatch/...
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (SD-1, AC-CR-022, T-009)
package dispatch

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
	"github.com/modu-ai/mink/internal/auth/file"
)

// controlledKeyringWrapper wraps a credential.Service and can be told to
// return ErrKeyringUnavailable for the next N calls.
type controlledKeyringWrapper struct {
	inner         credential.Service
	forceFailNext atomic.Int64 // if > 0, next call returns ErrKeyringUnavailable
}

func newControlledWrapper(inner credential.Service) *controlledKeyringWrapper {
	return &controlledKeyringWrapper{inner: inner}
}

// ForceFailNext instructs the wrapper to return ErrKeyringUnavailable for the
// next n calls.
func (c *controlledKeyringWrapper) ForceFailNext(n int64) {
	c.forceFailNext.Store(n)
}

func (c *controlledKeyringWrapper) maybeForce() bool {
	for {
		n := c.forceFailNext.Load()
		if n <= 0 {
			return false
		}
		if c.forceFailNext.CompareAndSwap(n, n-1) {
			return true
		}
	}
}

func (c *controlledKeyringWrapper) Store(provider string, cred credential.Credential) error {
	if c.maybeForce() {
		return credential.ErrKeyringUnavailable
	}
	return c.inner.Store(provider, cred)
}

func (c *controlledKeyringWrapper) Load(provider string) (credential.Credential, error) {
	if c.maybeForce() {
		return nil, credential.ErrKeyringUnavailable
	}
	return c.inner.Load(provider)
}

func (c *controlledKeyringWrapper) Delete(provider string) error {
	if c.maybeForce() {
		return credential.ErrKeyringUnavailable
	}
	return c.inner.Delete(provider)
}

func (c *controlledKeyringWrapper) List() ([]string, error) {
	if c.maybeForce() {
		return nil, credential.ErrKeyringUnavailable
	}
	return c.inner.List()
}

func (c *controlledKeyringWrapper) Health(provider string) (credential.HealthStatus, error) {
	if c.maybeForce() {
		return credential.HealthStatus{}, credential.ErrKeyringUnavailable
	}
	return c.inner.Health(provider)
}

// TestIntegrationDispatcherFallbackEndToEnd wires a real file.Backend (tmp dir)
// with a controlled keyring wrapper and asserts the fallback transition from
// keyring to file.
func TestIntegrationDispatcherFallbackEndToEnd(t *testing.T) {
	// Real file backend in a temp directory.
	dir := t.TempDir()
	fileBE, err := file.NewBackend(file.WithPath(filepath.Join(dir, "credentials.json")))
	if err != nil {
		t.Fatalf("file.NewBackend: %v", err)
	}

	// In-memory "keyring" stand-in — controlled wrapper forces unavailability.
	krInner := newMemBackend()
	krWrapper := newControlledWrapper(krInner)

	d, err := NewDispatcher(ModeKeyringFile, krWrapper, fileBE)
	if err != nil {
		t.Fatalf("NewDispatcher: %v", err)
	}

	cred := credential.APIKey{Value: "sk-integration-fallback-test"}

	// --- Phase 1: keyring succeeds (baseline) ---
	if err := d.Store("anthropic", cred); err != nil {
		t.Fatalf("Phase 1 Store: %v", err)
	}

	loaded, err := d.Load("anthropic")
	if err != nil {
		t.Fatalf("Phase 1 Load: %v", err)
	}
	if loaded.(credential.APIKey).Value != cred.Value {
		t.Errorf("Phase 1 Load: got %v, want %v", loaded, cred)
	}

	// --- Phase 2: force keyring unavailability on next Store ---
	krWrapper.ForceFailNext(1)

	cred2 := credential.APIKey{Value: "sk-fallback-written-to-file"}
	if err := d.Store("deepseek", cred2); err != nil {
		t.Fatalf("Phase 2 Store (fallback): %v", err)
	}

	// Dispatcher should have cached the unavailability flag after Phase 2.
	// Subsequent Load calls should go directly to file — krInner never sees them.
	prevKrCalls := krInner.callCount

	loaded2, err := d.Load("deepseek")
	if err != nil {
		t.Fatalf("Phase 2 Load: %v", err)
	}
	if loaded2.(credential.APIKey).Value != cred2.Value {
		t.Errorf("Phase 2 Load: got %v, want %v", loaded2, cred2)
	}

	// Keyring inner backend must not have been called after the transition.
	if krInner.callCount != prevKrCalls {
		t.Errorf("krInner.callCount changed after unavailability cache: was %d, now %d",
			prevKrCalls, krInner.callCount)
	}

	// --- Phase 3: List should work via the file backend ---
	ids, err := d.List()
	if err != nil {
		t.Fatalf("Phase 3 List: %v", err)
	}

	found := false
	for _, id := range ids {
		if id == "deepseek" {
			found = true
		}
	}
	if !found {
		t.Errorf("Phase 3 List: 'deepseek' not found in %v", ids)
	}

	// --- Phase 4: ErrNotFound propagates correctly ---
	_, err = d.Load("telegram_bot")
	if !errors.Is(err, credential.ErrNotFound) {
		t.Errorf("Phase 4 Load missing: expected ErrNotFound, got %v", err)
	}
}
