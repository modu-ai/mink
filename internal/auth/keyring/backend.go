// Package keyring wraps github.com/zalando/go-keyring to implement the
// credential.Service interface for MINK's OS keyring backend.
//
// Service identifier format: "mink:auth:{provider}"
// Account name:              "default" (single-account policy, UB-7, UN-3)
//
// All keyring calls are wrapped with a 5-second context timeout to prevent
// UI-unlock prompts from hanging in headless sessions (R8 mitigation).
//
// SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (UB-7, UB-8, UB-9, ED-1, ED-2, ED-3)
package keyring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	zKeyring "github.com/zalando/go-keyring"

	"github.com/modu-ai/mink/internal/auth/credential"
)

const (
	// accountName is the fixed account field used for every keyring entry.
	// The single-account policy (UB-7, UN-3) guarantees that only one
	// credential per provider is ever stored.
	accountName = "default"

	// keyringTimeout is the per-call timeout applied to every blocking OS
	// keyring API call to avoid hanging headless sessions (R8, plan.md §5).
	keyringTimeout = 5 * time.Second

	// indexProvider is the synthetic provider key used to track the list of
	// registered provider IDs.
	//
	// go-keyring does not expose enumeration, so Backend maintains a JSON
	// array of provider IDs in a separate keyring entry under this key.
	// Updated on every Store and Delete call.
	indexProvider = "_index"
)

// Backend implements credential.Service using the OS native keyring via
// github.com/zalando/go-keyring.
//
// @MX:ANCHOR: [AUTO] Backend.Store/Load/Delete/List/Health are the primary
// entry points called by the dispatch layer and tests (fan_in >= 3).
// @MX:REASON: All credential writes and reads funnel through this struct;
// a signature change here cascades to the dispatch layer and integration tests.
// @MX:SPEC: SPEC-MINK-AUTH-CREDENTIAL-001 (T-002, T-004, T-005)
type Backend struct{}

// NewBackend returns a zero-value Backend ready for use.
func NewBackend() *Backend {
	return &Backend{}
}

// serviceIdentifier returns the OS keyring service identifier for the given
// provider, following the "mink:auth:{provider}" format required by UB-7 and
// AC-CR-005.
func serviceIdentifier(provider string) string {
	return "mink:auth:" + provider
}

// callWithTimeout executes fn in a goroutine and returns its results when fn
// completes or when the 5-second keyring timeout elapses.
//
// @MX:WARN: [AUTO] Goroutine spawned for every keyring call.
// @MX:REASON: go-keyring is blocking and may hang indefinitely in headless
// SSH sessions when the OS presents an unlock dialog. The goroutine approach
// is the only way to impose a deadline without modifying go-keyring internals
// (R8, plan.md §5).
func callWithTimeout(fn func() (string, error)) (string, error) {
	type result struct {
		val string
		err error
	}
	ch := make(chan result, 1)
	go func() {
		val, err := fn()
		ch <- result{val, err}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), keyringTimeout)
	defer cancel()

	select {
	case r := <-ch:
		return r.val, r.err
	case <-ctx.Done():
		return "", fmt.Errorf("%w: keyring call timed out after %s",
			credential.ErrKeyringUnavailable, keyringTimeout)
	}
}

// callVoidWithTimeout is like callWithTimeout but for operations that return
// only an error (Set/Delete).
//
// @MX:WARN: [AUTO] Goroutine spawned for every void keyring call.
// @MX:REASON: Same reasoning as callWithTimeout — go-keyring blocking calls
// must be given a deadline to avoid headless session hangs (R8).
func callVoidWithTimeout(fn func() error) error {
	ch := make(chan error, 1)
	go func() {
		ch <- fn()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), keyringTimeout)
	defer cancel()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return fmt.Errorf("%w: keyring call timed out after %s",
			credential.ErrKeyringUnavailable, keyringTimeout)
	}
}

// Store serializes cred to JSON and writes it to the OS keyring entry
// "mink:auth:{provider}" / "default".
//
// Calling Store on an existing entry overwrites it (UB-4 single-account
// policy).  The internal provider index is updated atomically after the
// write.
func (b *Backend) Store(provider string, cred credential.Credential) error {
	if err := cred.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(marshalEnvelope{
		Kind:    string(cred.Kind()),
		Payload: cred,
	})
	if err != nil {
		return fmt.Errorf("keyring: marshal credential: %w", err)
	}

	svc := serviceIdentifier(provider)
	if err := callVoidWithTimeout(func() error {
		return zKeyring.Set(svc, accountName, string(data))
	}); err != nil {
		return wrapKeyringErr(err)
	}

	return b.indexAdd(provider)
}

// Load retrieves the credential for the given provider from the OS keyring.
// Returns credential.ErrNotFound when no entry exists.
// Returns credential.ErrKeyringUnavailable when the OS keyring is not
// reachable.
func (b *Backend) Load(provider string) (credential.Credential, error) {
	svc := serviceIdentifier(provider)
	val, err := callWithTimeout(func() (string, error) {
		return zKeyring.Get(svc, accountName)
	})
	if err != nil {
		return nil, wrapKeyringErr(err)
	}

	return unmarshalCredential([]byte(val))
}

// Delete removes the credential for the given provider from the OS keyring.
// Delete is idempotent: ErrNotFound from the OS is silently swallowed (ED-3).
func (b *Backend) Delete(provider string) error {
	svc := serviceIdentifier(provider)
	err := callVoidWithTimeout(func() error {
		return zKeyring.Delete(svc, accountName)
	})
	if err != nil && !errors.Is(err, zKeyring.ErrNotFound) {
		return wrapKeyringErr(err)
	}

	// Remove from the index regardless of whether the entry existed.
	return b.indexRemove(provider)
}

// List returns the provider IDs that have a stored credential entry.
//
// Because go-keyring does not expose enumeration, an auxiliary index entry
// under "mink:auth:_index" / "default" stores a JSON array of provider IDs.
// The index is kept consistent by Store and Delete.
func (b *Backend) List() ([]string, error) {
	return b.indexGet()
}

// Health probes the presence and masked value of the credential for the
// given provider without leaking plaintext (UB-8).
func (b *Backend) Health(provider string) (credential.HealthStatus, error) {
	cred, err := b.Load(provider)
	if err != nil {
		if errors.Is(err, credential.ErrNotFound) {
			return credential.HealthStatus{
				Present: false,
				Backend: "keyring",
			}, nil
		}
		return credential.HealthStatus{Backend: "keyring"}, err
	}

	return credential.HealthStatus{
		Present:     true,
		MaskedLast4: cred.MaskedString(),
		Backend:     "keyring",
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// marshalEnvelope wraps a Credential for JSON serialisation.  The Kind field
// is stored alongside the payload so that unmarshalCredential can reconstruct
// the concrete type without requiring type-switches on the caller side.
type marshalEnvelope struct {
	Kind    string                `json:"kind"`
	Payload credential.Credential `json:"payload"`
}

// apiKeyPayload is used to unmarshal the JSON payload for KindAPIKey entries.
type apiKeyPayload struct {
	Value string `json:"Value"`
}

// unmarshalCredential decodes a JSON-encoded credential blob produced by
// Store.  Currently handles KindAPIKey only (M1 scope); other kinds are added
// in M3 T-013.
func unmarshalCredential(data []byte) (credential.Credential, error) {
	// First pass: decode just the kind discriminator.
	var env struct {
		Kind    string          `json:"kind"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("keyring: unmarshal envelope: %w", err)
	}

	switch credential.Kind(env.Kind) {
	case credential.KindAPIKey:
		var p apiKeyPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			return nil, fmt.Errorf("keyring: unmarshal api_key payload: %w", err)
		}
		return credential.APIKey{Value: p.Value}, nil
	default:
		return nil, fmt.Errorf("keyring: unknown credential kind %q: %w",
			env.Kind, credential.ErrSchemaViolation)
	}
}

// wrapKeyringErr converts go-keyring sentinel errors to MINK sentinels.
func wrapKeyringErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, zKeyring.ErrNotFound) {
		return fmt.Errorf("%w: %s", credential.ErrNotFound, err)
	}
	// Any other error from go-keyring (D-Bus failure, macOS
	// errSecInteractionNotAllowed, Wincred error, or our own timeout) is
	// treated as keyring unavailability.
	if errors.Is(err, credential.ErrKeyringUnavailable) {
		return err
	}
	return fmt.Errorf("%w: %s", credential.ErrKeyringUnavailable, err)
}

// ---------------------------------------------------------------------------
// Index helpers — JSON array of registered provider IDs stored at
// "mink:auth:_index" / "default".
// ---------------------------------------------------------------------------

func (b *Backend) indexGet() ([]string, error) {
	svc := serviceIdentifier(indexProvider)
	raw, err := callWithTimeout(func() (string, error) {
		return zKeyring.Get(svc, accountName)
	})
	if err != nil {
		// No index entry yet → empty list, not an error.
		if errors.Is(err, zKeyring.ErrNotFound) ||
			errors.Is(err, credential.ErrNotFound) {
			return []string{}, nil
		}
		return nil, wrapKeyringErr(err)
	}

	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		// Corrupted index: return empty rather than propagating.
		return []string{}, nil
	}
	return ids, nil
}

func (b *Backend) indexAdd(provider string) error {
	ids, err := b.indexGet()
	if err != nil {
		return err
	}
	if slices.Contains(ids, provider) {
		return nil // already in index
	}
	ids = append(ids, provider)
	return b.indexSet(ids)
}

func (b *Backend) indexRemove(provider string) error {
	ids, err := b.indexGet()
	if err != nil {
		return err
	}
	filtered := ids[:0]
	for _, id := range ids {
		if id != provider {
			filtered = append(filtered, id)
		}
	}
	return b.indexSet(filtered)
}

func (b *Backend) indexSet(ids []string) error {
	data, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("keyring: marshal index: %w", err)
	}
	svc := serviceIdentifier(indexProvider)
	if err := callVoidWithTimeout(func() error {
		return zKeyring.Set(svc, accountName, string(data))
	}); err != nil {
		return wrapKeyringErr(err)
	}
	return nil
}
