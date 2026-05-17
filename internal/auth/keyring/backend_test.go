package keyring_test

import (
	"errors"
	"slices"
	"testing"

	zKeyring "github.com/zalando/go-keyring"

	"github.com/modu-ai/mink/internal/auth/credential"
	"github.com/modu-ai/mink/internal/auth/keyring"
)

// mockInit activates go-keyring's in-memory mock and resets it to a clean
// state.  Must be called at the start of each test because the mock is a
// global singleton and tests may leave entries behind.
//
// NOTE: Tests in this package must NOT run in parallel; the go-keyring mock
// is a non-concurrent global map and races across parallel tests will produce
// false failures under -race.
func mockInit() {
	zKeyring.MockInit()
}

func TestServiceIdentifierFormat(t *testing.T) {
	mockInit()

	tests := []struct {
		provider string
		wantSvc  string
	}{
		{provider: "anthropic", wantSvc: "mink:auth:anthropic"},
		{provider: "codex", wantSvc: "mink:auth:codex"},
		{provider: "slack", wantSvc: "mink:auth:slack"},
	}

	b := keyring.NewBackend()

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			// Re-init for each sub-test to ensure clean state.
			mockInit()

			key := credential.APIKey{Value: "test-value-for-" + tc.provider}
			if err := b.Store(tc.provider, key); err != nil {
				t.Fatalf("Store(%q): %v", tc.provider, err)
			}

			got, err := b.Load(tc.provider)
			if err != nil {
				t.Fatalf("Load(%q): %v", tc.provider, err)
			}
			apiKey, ok := got.(credential.APIKey)
			if !ok {
				t.Fatalf("Load(%q): expected APIKey, got %T", tc.provider, got)
			}
			if apiKey.Value != key.Value {
				t.Errorf("Load(%q).Value = %q; want %q", tc.provider, apiKey.Value, key.Value)
			}

			// Account must be "default" — verified by checking that the entry
			// is reachable via the mock keyring using the raw account value.
			rawVal, err := zKeyring.Get(tc.wantSvc, "default")
			if err != nil {
				t.Errorf("zKeyring.Get(%q, \"default\"): %v — account name may be wrong", tc.wantSvc, err)
			}
			if rawVal == "" {
				t.Errorf("zKeyring.Get(%q, \"default\") returned empty value", tc.wantSvc)
			}
		})
	}
}

func TestStoreLoadDeleteRoundTrip(t *testing.T) {
	mockInit()

	b := keyring.NewBackend()
	const provider = "anthropic"

	// Store
	original := credential.APIKey{Value: "sk-ant-round-trip-test"}
	if err := b.Store(provider, original); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// Load
	got, err := b.Load(provider)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	apiKey, ok := got.(credential.APIKey)
	if !ok {
		t.Fatalf("Load returned %T, want APIKey", got)
	}
	if apiKey.Value != original.Value {
		t.Errorf("loaded value %q; want %q", apiKey.Value, original.Value)
	}

	// List — must include provider
	ids, err := b.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !slices.Contains(ids, provider) {
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

	// List after delete must not include provider
	ids2, err := b.List()
	if err != nil {
		t.Fatalf("List after Delete: %v", err)
	}
	if slices.Contains(ids2, provider) {
		t.Errorf("List() after Delete still contains %q", provider)
	}
}

func TestDeleteIdempotent(t *testing.T) {
	mockInit()

	b := keyring.NewBackend()

	// Deleting a non-existent provider must not return an error.
	if err := b.Delete("nonexistent-provider"); err != nil {
		t.Errorf("Delete(nonexistent) returned error: %v", err)
	}

	// Second delete on an already-deleted provider must also succeed.
	_ = b.Store("idempotent-test", credential.APIKey{Value: "temp"})
	_ = b.Delete("idempotent-test")
	if err := b.Delete("idempotent-test"); err != nil {
		t.Errorf("second Delete returned error: %v", err)
	}
}

func TestStoreOverwritesSingleSlot(t *testing.T) {
	mockInit()

	b := keyring.NewBackend()
	const provider = "deepseek"

	_ = b.Store(provider, credential.APIKey{Value: "key-v1"})
	_ = b.Store(provider, credential.APIKey{Value: "key-v2"})

	got, err := b.Load(provider)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if apiKey, ok := got.(credential.APIKey); !ok || apiKey.Value != "key-v2" {
		t.Errorf("overwrite: expected key-v2, got %+v", got)
	}

	// List must still show a single entry for this provider.
	ids, _ := b.List()
	count := 0
	for _, id := range ids {
		if id == provider {
			count++
		}
	}
	if count != 1 {
		t.Errorf("List() has %d entries for %q; want exactly 1", count, provider)
	}
}

func TestHealthPlaintextAbsent(t *testing.T) {
	mockInit()

	b := keyring.NewBackend()
	const provider = "health-test-prov"
	const secretValue = "sk-ant-1234567890"

	_ = b.Store(provider, credential.APIKey{Value: secretValue})

	status, err := b.Health(provider)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !status.Present {
		t.Error("Health.Present = false; want true")
	}
	if status.Backend != "keyring" {
		t.Errorf("Health.Backend = %q; want \"keyring\"", status.Backend)
	}
	// MaskedLast4 must not contain the full secret.
	if status.MaskedLast4 == secretValue {
		t.Errorf("Health.MaskedLast4 = %q; want masked value, not plaintext", status.MaskedLast4)
	}
	// Must start with ***.
	if len(status.MaskedLast4) < 3 || status.MaskedLast4[:3] != "***" {
		t.Errorf("Health.MaskedLast4 = %q; expected *** prefix", status.MaskedLast4)
	}
}

func TestHealthMissingCredential(t *testing.T) {
	mockInit()

	b := keyring.NewBackend()
	status, err := b.Health("no-such-provider-xyz")
	if err != nil {
		t.Fatalf("Health returned unexpected error for missing provider: %v", err)
	}
	if status.Present {
		t.Error("Health.Present = true for missing provider; want false")
	}
	if status.Backend != "keyring" {
		t.Errorf("Health.Backend = %q; want \"keyring\"", status.Backend)
	}
}
