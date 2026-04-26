// Package credential — OpenAI Codex credential source.
//
// SPEC-GOOSE-CREDPOOL-001 OI-05.
//
// Reads ~/.codex/auth.json (the file OpenAI's Codex CLI writes after sign-in)
// and emits a metadata-only PooledCredential. The file may carry either an
// OAuth tokens block, a static API key, or both; one entry is returned per
// file, preferring the OAuth tokens when present (they carry an expiry).
package credential

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// OpenAICodexSource reads OpenAI Codex's auth.json file.
//
// @MX:NOTE: [AUTO] Vendor file reader for ~/.codex/auth.json.
// @MX:SPEC: SPEC-GOOSE-CREDPOOL-001 OI-05 / AC-CREDPOOL-007
type OpenAICodexSource struct {
	path       string
	keyringRef string
}

// openaiCodexFile mirrors the on-disk schema. Vendor-managed.
type openaiCodexFile struct {
	OpenAIAPIKey *string `json:"OPENAI_API_KEY"`
	Tokens       *struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
	} `json:"tokens"`
	LastRefresh string `json:"last_refresh"` // RFC3339; advisory metadata
}

// NewOpenAICodexSource constructs a source backed by the given file path.
func NewOpenAICodexSource(path string) *OpenAICodexSource {
	return &OpenAICodexSource{path: path}
}

// WithOpenAIKeyringRef overrides the KeyringID label.
func (s *OpenAICodexSource) WithOpenAIKeyringRef(ref string) *OpenAICodexSource {
	s.keyringRef = ref
	return s
}

// DefaultOpenAICodexCredentialsPath returns ~/.codex/auth.json.
func DefaultOpenAICodexCredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".codex", "auth.json")
	}
	return filepath.Join(home, ".codex", "auth.json")
}

// Load reads the credential file and returns a single metadata-only entry.
// Missing files return (nil, nil); malformed JSON returns a wrapped error.
// A file containing neither OAuth tokens nor an API key returns (nil, nil).
func (s *OpenAICodexSource) Load(ctx context.Context) ([]*PooledCredential, error) {
	data, err := loadVendorFile(ctx, s.path)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var parsed openaiCodexFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, wrapVendorParseError("openai", s.path, err)
	}

	hasTokens := parsed.Tokens != nil &&
		(parsed.Tokens.AccessToken != "" || parsed.Tokens.RefreshToken != "")
	hasAPIKey := parsed.OpenAIAPIKey != nil && *parsed.OpenAIAPIKey != ""

	if !hasTokens && !hasAPIKey {
		return nil, nil
	}

	keyringRef := s.keyringRef
	if keyringRef == "" {
		keyringRef = "openai-codex-default"
	}

	// OAuth tokens block carries the implicit expiry. Codex auth.json does
	// NOT publish an explicit `expires_at`, so we treat the entry as
	// permanently valid (zero ExpiresAt) and let the upstream Refresher
	// decide when to rotate based on transport-layer 401 feedback.
	// API-key-only entries are also permanently valid.
	expires := time.Time{}

	cred := &PooledCredential{
		ID:        keyringRef,
		Provider:  "openai",
		KeyringID: keyringRef,
		Status:    statusForExpiry(expires, time.Now()),
		ExpiresAt: expires,
	}

	// Defensive zeroing.
	if parsed.Tokens != nil {
		parsed.Tokens.AccessToken = ""
		parsed.Tokens.RefreshToken = ""
		parsed.Tokens.IDToken = ""
	}
	if parsed.OpenAIAPIKey != nil {
		empty := ""
		parsed.OpenAIAPIKey = &empty
	}

	return []*PooledCredential{cred}, nil
}
