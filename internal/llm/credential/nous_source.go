// Package credential — Nous Hermes credential source.
//
// SPEC-GOOSE-CREDPOOL-001 OI-05.
//
// Reads ~/.hermes/auth.json (the file Nous's Hermes Portal CLI writes after
// portal login) and emits a metadata-only PooledCredential. The agent_key
// has a TTL expressed as RFC3339 in the expires_at field; we propagate the
// expiry to PooledCredential.ExpiresAt so the pool can apply the standard
// expiration filter (REQ-CREDPOOL-001 b).
package credential

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// NousSource reads Nous Hermes's auth.json file.
//
// @MX:NOTE: [AUTO] Vendor file reader for ~/.hermes/auth.json.
// @MX:SPEC: SPEC-GOOSE-CREDPOOL-001 OI-05 / AC-CREDPOOL-007
type NousSource struct {
	path       string
	keyringRef string
}

// nousHermesFile mirrors the on-disk schema. Vendor-managed.
type nousHermesFile struct {
	AgentKey  string `json:"agent_key"`
	ExpiresAt string `json:"expires_at"` // RFC3339; optional
}

// NewNousSource constructs a source backed by the given file path.
func NewNousSource(path string) *NousSource {
	return &NousSource{path: path}
}

// WithNousKeyringRef overrides the KeyringID label.
func (s *NousSource) WithNousKeyringRef(ref string) *NousSource {
	s.keyringRef = ref
	return s
}

// DefaultNousCredentialsPath returns ~/.hermes/auth.json.
func DefaultNousCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".hermes", "auth.json")
	}
	return filepath.Join(home, ".hermes", "auth.json")
}

// Load reads the credential file and returns a metadata-only entry.
// Missing files return (nil, nil); malformed JSON returns a wrapped error.
// A file without an agent_key returns (nil, nil).
func (s *NousSource) Load(ctx context.Context) ([]*PooledCredential, error) {
	data, err := loadVendorFile(ctx, s.path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var parsed nousHermesFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, wrapVendorParseError("nous", s.path, err)
	}

	if parsed.AgentKey == "" {
		return nil, nil
	}

	expires := time.Time{}
	if parsed.ExpiresAt != "" {
		if t, perr := time.Parse(time.RFC3339, parsed.ExpiresAt); perr == nil {
			expires = t
		}
		// A malformed timestamp is treated as "no expiry given" — preferable
		// to failing the entire load over a non-essential field.
	}

	keyringRef := s.keyringRef
	if keyringRef == "" {
		keyringRef = "nous-hermes-default"
	}

	cred := &PooledCredential{
		ID:        keyringRef,
		Provider:  "nous",
		KeyringID: keyringRef,
		Status:    statusForExpiry(expires, time.Now()),
		ExpiresAt: expires,
	}

	// Defensive zeroing.
	parsed.AgentKey = ""

	return []*PooledCredential{cred}, nil
}
