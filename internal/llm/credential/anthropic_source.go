// Package credential — Anthropic Claude credential source.
//
// SPEC-GOOSE-CREDPOOL-001 OI-05.
//
// Reads ~/.claude/.credentials.json (the file Anthropic's own Claude Code
// CLI writes after PKCE login) and emits a metadata-only PooledCredential.
// Raw OAuth tokens parsed from the file are dropped before Load() returns,
// preserving the Zero-Knowledge invariant (REQ-CREDPOOL-014).
package credential

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// AnthropicClaudeSource reads Anthropic's Claude Code credential file and
// emits a single metadata-only PooledCredential.
//
// @MX:NOTE: [AUTO] Vendor file reader for ~/.claude/.credentials.json.
// @MX:SPEC: SPEC-GOOSE-CREDPOOL-001 OI-05 / AC-CREDPOOL-007
type AnthropicClaudeSource struct {
	path       string
	keyringRef string // optional override; if empty a default label is used
}

// anthropicClaudeFile mirrors the on-disk schema. Vendor-managed: do NOT
// extend without checking Anthropic's CLI release notes.
type anthropicClaudeFile struct {
	ClaudeAIOAuth struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresAt    int64  `json:"expiresAt"` // ms epoch
	} `json:"claudeAiOauth"`
}

// NewAnthropicClaudeSource constructs a source backed by the given file path.
// The path is injectable to keep tests hermetic; production callers typically
// pass DefaultAnthropicClaudeCredentialsPath().
func NewAnthropicClaudeSource(path string) *AnthropicClaudeSource {
	return &AnthropicClaudeSource{path: path}
}

// WithAnthropicKeyringRef overrides the KeyringID label emitted on the
// resulting PooledCredential. Useful when a user maintains multiple Anthropic
// accounts and wants distinct keyring entries.
func (s *AnthropicClaudeSource) WithAnthropicKeyringRef(ref string) *AnthropicClaudeSource {
	s.keyringRef = ref
	return s
}

// DefaultAnthropicClaudeCredentialsPath returns ~/.claude/.credentials.json
// resolved against the current user's home directory. Returns the bare
// filename when the home directory cannot be determined (a degraded but
// non-panicking fallback).
func DefaultAnthropicClaudeCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".claude", ".credentials.json")
	}
	return filepath.Join(home, ".claude", ".credentials.json")
}

// Load reads the credential file and returns a metadata-only entry.
// Missing files return (nil, nil); malformed JSON returns a wrapped error.
func (s *AnthropicClaudeSource) Load(ctx context.Context) ([]*PooledCredential, error) {
	data, err := loadVendorFile(ctx, s.path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var parsed anthropicClaudeFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, wrapVendorParseError("anthropic", s.path, err)
	}

	// No OAuth block = nothing to expose.
	if parsed.ClaudeAIOAuth.AccessToken == "" && parsed.ClaudeAIOAuth.RefreshToken == "" {
		return nil, nil
	}

	expires := time.Time{}
	if parsed.ClaudeAIOAuth.ExpiresAt > 0 {
		expires = time.UnixMilli(parsed.ClaudeAIOAuth.ExpiresAt)
	}

	keyringRef := s.keyringRef
	if keyringRef == "" {
		keyringRef = "anthropic-claude-default"
	}

	cred := &PooledCredential{
		ID:        keyringRef,
		Provider:  "anthropic",
		KeyringID: keyringRef,
		Status:    statusForExpiry(expires, time.Now()),
		ExpiresAt: expires,
	}

	// Defensive zeroing: secrets only ever lived inside `parsed`, which goes
	// out of scope at function return. We intentionally do NOT copy any token
	// bytes into the returned struct.
	parsed.ClaudeAIOAuth.AccessToken = ""
	parsed.ClaudeAIOAuth.RefreshToken = ""

	return []*PooledCredential{cred}, nil
}
