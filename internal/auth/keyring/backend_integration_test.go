//go:build integration

// Package keyring — integration test for the keyring backend.
//
// This test requires a real OS keyring daemon and must be run with the
// MINK_INTEGRATION_KEYRING=1 environment variable set.  On Linux, a running
// gnome-keyring-daemon with DBUS_SESSION_BUS_ADDRESS set is required.
//
// Usage:
//
//	MINK_INTEGRATION_KEYRING=1 go test -tags=integration -v ./internal/auth/keyring/...
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (T-005, AC-CR-002, AC-CR-009, AC-CR-015)
package keyring_test

import (
	"errors"
	"os"
	"testing"

	"github.com/modu-ai/mink/internal/auth/credential"
	"github.com/modu-ai/mink/internal/auth/keyring"
)

// TestIntegrationRoundTrip performs a Store → Load → List → Delete → Load
// round-trip against the real OS keyring.
//
// AC-CR-002 (UB-2 keyring write path), AC-CR-009 (UB-9 cross-platform),
// AC-CR-015 (ED-3 logout idempotent).
func TestIntegrationRoundTrip(t *testing.T) {
	if os.Getenv("MINK_INTEGRATION_KEYRING") != "1" {
		t.Skip("set MINK_INTEGRATION_KEYRING=1 to run keyring integration tests")
	}

	b := keyring.NewBackend()
	const provider = "anthropic"
	const secretValue = "sk-ant-integration-test-0000"

	// Store
	if err := b.Store(provider, credential.APIKey{Value: secretValue}); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Load — value must match
	got, err := b.Load(provider)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	apiKey, ok := got.(credential.APIKey)
	if !ok {
		t.Fatalf("Load returned %T, want APIKey", got)
	}
	if apiKey.Value != secretValue {
		t.Errorf("Load value %q; want %q", apiKey.Value, secretValue)
	}

	// List — must contain 1 entry with our provider
	ids, err := b.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !containsString(ids, provider) {
		t.Errorf("List() = %v; want to include %q", ids, provider)
	}

	// Delete
	if err := b.Delete(provider); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Load after delete must return ErrNotFound
	_, err = b.Load(provider)
	if !errors.Is(err, credential.ErrNotFound) {
		t.Errorf("Load after Delete returned %v; want ErrNotFound", err)
	}

	// List after delete must be empty (or not include our provider)
	ids2, err := b.List()
	if err != nil {
		t.Fatalf("List after Delete: %v", err)
	}
	if containsString(ids2, provider) {
		t.Errorf("List() after Delete still contains %q: %v", provider, ids2)
	}

	// Second Delete must be idempotent (no error)
	if err := b.Delete(provider); err != nil {
		t.Errorf("second Delete returned error: %v", err)
	}
}
