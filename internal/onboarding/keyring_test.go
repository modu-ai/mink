// Package onboarding — keyring_test.go tests InMemoryKeyring and the high-level
// provider API key helpers (SetProviderAPIKey, GetProviderAPIKey, DeleteProviderAPIKey).
//
// SystemKeyring is NOT tested here: it requires a functioning OS keyring backend
// (macOS Keychain, Linux Secret Service, Windows Credential Manager), which is
// unavailable on headless CI runners. SystemKeyring integration tests are left
// for a future PR gated behind `//go:build integration`.
//
// SPEC: SPEC-MINK-ONBOARDING-001 §6.6
// REQ: REQ-OB-007
package onboarding

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

// --- InMemoryKeyring low-level tests ---

// TestInMemoryKeyring_SetGetDelete_RoundTrip verifies the basic Set → Get → Delete
// lifecycle of the in-memory backend.
func TestInMemoryKeyring_SetGetDelete_RoundTrip(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()

	if err := c.Set("foo", "bar"); err != nil {
		t.Fatalf("Set: unexpected error: %v", err)
	}

	got, err := c.Get("foo")
	if err != nil {
		t.Fatalf("Get after Set: unexpected error: %v", err)
	}
	if got != "bar" {
		t.Errorf("Get: got %q, want %q", got, "bar")
	}

	if err := c.Delete("foo"); err != nil {
		t.Fatalf("Delete: unexpected error: %v", err)
	}

	_, err = c.Get("foo")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get after Delete: got error %v, want ErrKeyNotFound", err)
	}
}

// TestInMemoryKeyring_Get_MissingKey_ReturnsErrKeyNotFound confirms that Get on
// an empty store returns ErrKeyNotFound.
func TestInMemoryKeyring_Get_MissingKey_ReturnsErrKeyNotFound(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()

	_, err := c.Get("absent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("Get on empty store: got error %v, want ErrKeyNotFound", err)
	}
}

// TestInMemoryKeyring_Set_Overwrites verifies that a second Set replaces the
// value from the first Set.
func TestInMemoryKeyring_Set_Overwrites(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()

	if err := c.Set("key", "v1"); err != nil {
		t.Fatalf("first Set: %v", err)
	}
	if err := c.Set("key", "v2"); err != nil {
		t.Fatalf("second Set: %v", err)
	}

	got, err := c.Get("key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "v2" {
		t.Errorf("Get after overwrite: got %q, want %q", got, "v2")
	}
}

// TestInMemoryKeyring_Delete_Idempotent confirms that Delete on a non-existent
// key returns nil without error.
func TestInMemoryKeyring_Delete_Idempotent(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()

	if err := c.Delete("does-not-exist"); err != nil {
		t.Errorf("Delete on absent key: got error %v, want nil", err)
	}
}

// TestInMemoryKeyring_ConcurrentSafe spawns 100 goroutines performing
// Set/Get/Delete operations on disjoint keys and verifies that the final
// state is consistent with no data races.
// Run with: go test -race ./internal/onboarding/...
func TestInMemoryKeyring_ConcurrentSafe(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)

	for i := range n {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent-key-%d", i)
			val := fmt.Sprintf("concurrent-val-%d", i)

			if err := c.Set(key, val); err != nil {
				t.Errorf("goroutine %d Set: %v", i, err)
				return
			}
			got, err := c.Get(key)
			if err != nil {
				t.Errorf("goroutine %d Get: %v", i, err)
				return
			}
			if got != val {
				t.Errorf("goroutine %d Get: got %q, want %q", i, got, val)
			}
			if err := c.Delete(key); err != nil {
				t.Errorf("goroutine %d Delete: %v", i, err)
			}
		}(i)
	}

	wg.Wait()

	// After all goroutines complete, all keys should be deleted.
	for i := range n {
		key := fmt.Sprintf("concurrent-key-%d", i)
		_, err := c.Get(key)
		if !errors.Is(err, ErrKeyNotFound) {
			t.Errorf("after concurrent cleanup: key %q still present", key)
		}
	}
}

// --- providerEntryKey unit tests ---

// TestProviderEntryKey_NormalizesName verifies that providerEntryKey trims and
// lowercases the provider name and produces the canonical "provider.X.api_key" format.
func TestProviderEntryKey_NormalizesName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input   string
		want    string
		wantErr error
	}{
		{"Anthropic", "provider.anthropic.api_key", nil},
		{"  openai  ", "provider.openai.api_key", nil},
		{"GOOGLE", "provider.google.api_key", nil},
		{"", "", ErrInvalidProviderName},
		{"   ", "", ErrInvalidProviderName},
	}

	for _, tc := range cases {
		got, err := providerEntryKey(tc.input)
		if tc.wantErr != nil {
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("providerEntryKey(%q): got error %v, want %v", tc.input, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("providerEntryKey(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("providerEntryKey(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- SetProviderAPIKey tests ---

// TestSetProviderAPIKey_NilClient_Errors confirms that a nil KeyringClient
// returns ErrNilKeyringClient.
func TestSetProviderAPIKey_NilClient_Errors(t *testing.T) {
	t.Parallel()
	err := SetProviderAPIKey(nil, "anthropic", "sk-ant-test")
	if !errors.Is(err, ErrNilKeyringClient) {
		t.Errorf("got %v, want ErrNilKeyringClient", err)
	}
}

// TestSetProviderAPIKey_EmptyKey_Errors verifies that an empty API key returns
// ErrKeyringEmptyAPIKey (distinct from validators.go's ErrEmptyAPIKey).
func TestSetProviderAPIKey_EmptyKey_Errors(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()
	err := SetProviderAPIKey(c, "anthropic", "")
	if !errors.Is(err, ErrKeyringEmptyAPIKey) {
		t.Errorf("got %v, want ErrKeyringEmptyAPIKey", err)
	}
}

// TestSetProviderAPIKey_EmptyProvider_Errors verifies that an empty provider name
// returns ErrInvalidProviderName.
func TestSetProviderAPIKey_EmptyProvider_Errors(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()
	err := SetProviderAPIKey(c, "", "sk-ant-test")
	if !errors.Is(err, ErrInvalidProviderName) {
		t.Errorf("got %v, want ErrInvalidProviderName", err)
	}
}

// TestSetProviderAPIKey_RoundTripsViaGet confirms that SetProviderAPIKey stores
// the value that GetProviderAPIKey subsequently retrieves.
func TestSetProviderAPIKey_RoundTripsViaGet(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()
	const provider = "anthropic"
	const apiKey = "sk-ant-testkey12345"

	if err := SetProviderAPIKey(c, provider, apiKey); err != nil {
		t.Fatalf("SetProviderAPIKey: %v", err)
	}

	got, err := GetProviderAPIKey(c, provider)
	if err != nil {
		t.Fatalf("GetProviderAPIKey: %v", err)
	}
	if got != apiKey {
		t.Errorf("GetProviderAPIKey: got %q, want %q", got, apiKey)
	}
}

// --- GetProviderAPIKey tests ---

// TestGetProviderAPIKey_NilClient_Errors confirms nil client returns ErrNilKeyringClient.
func TestGetProviderAPIKey_NilClient_Errors(t *testing.T) {
	t.Parallel()
	_, err := GetProviderAPIKey(nil, "anthropic")
	if !errors.Is(err, ErrNilKeyringClient) {
		t.Errorf("got %v, want ErrNilKeyringClient", err)
	}
}

// TestGetProviderAPIKey_MissingProvider_Errors verifies that retrieving a provider
// key that was never stored returns ErrKeyNotFound.
func TestGetProviderAPIKey_MissingProvider_Errors(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()
	_, err := GetProviderAPIKey(c, "openai")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("got %v, want ErrKeyNotFound", err)
	}
}

// --- DeleteProviderAPIKey tests ---

// TestDeleteProviderAPIKey_RemovesEntry verifies that after Set + Delete, Get
// returns ErrKeyNotFound.
func TestDeleteProviderAPIKey_RemovesEntry(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()

	if err := SetProviderAPIKey(c, "google", "AIzaTestKey12345678901234567890123456789"); err != nil {
		t.Fatalf("SetProviderAPIKey: %v", err)
	}
	if err := DeleteProviderAPIKey(c, "google"); err != nil {
		t.Fatalf("DeleteProviderAPIKey: %v", err)
	}
	_, err := GetProviderAPIKey(c, "google")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("GetProviderAPIKey after delete: got %v, want ErrKeyNotFound", err)
	}
}

// TestDeleteProviderAPIKey_AbsentProvider_Idempotent confirms that deleting a
// provider key that was never stored returns nil.
func TestDeleteProviderAPIKey_AbsentProvider_Idempotent(t *testing.T) {
	t.Parallel()
	c := NewInMemoryKeyring()
	if err := DeleteProviderAPIKey(c, "deepseek"); err != nil {
		t.Errorf("DeleteProviderAPIKey on absent entry: got %v, want nil", err)
	}
}

// TestDeleteProviderAPIKey_NilClient_Errors confirms nil client returns ErrNilKeyringClient.
func TestDeleteProviderAPIKey_NilClient_Errors(t *testing.T) {
	t.Parallel()
	err := DeleteProviderAPIKey(nil, "anthropic")
	if !errors.Is(err, ErrNilKeyringClient) {
		t.Errorf("got %v, want ErrNilKeyringClient", err)
	}
}
