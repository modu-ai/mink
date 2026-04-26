// Package credential_test covers NewPoolsFromConfig wiring for
// SPEC-GOOSE-CREDPOOL-001 OI-06 (CONFIG-001 dependency).
package credential_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/config"
	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// makeAnthropicFile writes a minimal valid Anthropic credentials file.
func makeAnthropicFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".credentials.json")
	body := `{"claudeAiOauth":{"accessToken":"a","refreshToken":"r","expiresAt":` +
		itoa64(time.Now().Add(2*time.Hour).UnixMilli()) + `}}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// makeOpenAIFile writes a minimal valid OpenAI codex auth file.
func makeOpenAIFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	body := `{"OPENAI_API_KEY":"sk-x"}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// makeNousFile writes a minimal valid Nous hermes auth file.
func makeNousFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	body := `{"agent_key":"k"}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

// TestNewPoolsFromConfig_ThreeProviders_OnePoolEach verifies that three
// providers each declaring exactly one credential entry produce three
// independent pools, each with one selectable credential.
func TestNewPoolsFromConfig_ThreeProviders_OnePoolEach(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Providers: map[string]config.ProviderConfig{
				"anthropic": {
					Credentials: []config.CredentialConfig{
						{Type: "anthropic_claude_file", Path: makeAnthropicFile(t), KeyringRef: "anth-1"},
					},
				},
				"openai": {
					Credentials: []config.CredentialConfig{
						{Type: "openai_codex_file", Path: makeOpenAIFile(t), KeyringRef: "oai-1"},
					},
				},
				"nous": {
					Credentials: []config.CredentialConfig{
						{Type: "nous_hermes_file", Path: makeNousFile(t), KeyringRef: "nous-1"},
					},
				},
			},
		},
	}

	pools, err := credential.NewPoolsFromConfig(context.Background(), cfg, zap.NewNop())
	require.NoError(t, err)
	assert.Len(t, pools, 3)

	for name, pool := range pools {
		require.NotNil(t, pool, "pool for %q must not be nil", name)
		total, available := pool.Size()
		assert.Equal(t, 1, total, "provider %q total", name)
		assert.Equal(t, 1, available, "provider %q available", name)
	}
}

// TestNewPoolsFromConfig_MultipleCredentialsPerProvider_SingleCombinedPool
// verifies that when one provider declares multiple credential entries,
// they are merged into a single pool via an internal multi-source.
func TestNewPoolsFromConfig_MultipleCredentialsPerProvider_SingleCombinedPool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Providers: map[string]config.ProviderConfig{
				"anthropic": {
					Credentials: []config.CredentialConfig{
						{Type: "anthropic_claude_file", Path: makeAnthropicFile(t), KeyringRef: "anth-a"},
						{Type: "anthropic_claude_file", Path: makeAnthropicFile(t), KeyringRef: "anth-b"},
					},
				},
			},
		},
	}

	pools, err := credential.NewPoolsFromConfig(context.Background(), cfg, zap.NewNop())
	require.NoError(t, err)
	require.Len(t, pools, 1)

	pool := pools["anthropic"]
	require.NotNil(t, pool)
	total, available := pool.Size()
	assert.Equal(t, 2, total, "combined source should yield 2 entries")
	assert.Equal(t, 2, available, "both entries should be selectable")
}

// TestNewPoolsFromConfig_ProviderWithoutCredentials_Skipped verifies that
// providers declaring an empty credentials slice are silently dropped from
// the resulting pool map (per OI-06 constraint 2).
func TestNewPoolsFromConfig_ProviderWithoutCredentials_Skipped(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Providers: map[string]config.ProviderConfig{
				"ollama":    {Host: "http://localhost:11434"}, // no credentials
				"anthropic": {Credentials: []config.CredentialConfig{{Type: "anthropic_claude_file", Path: makeAnthropicFile(t), KeyringRef: "anth-1"}}},
			},
		},
	}

	pools, err := credential.NewPoolsFromConfig(context.Background(), cfg, zap.NewNop())
	require.NoError(t, err)
	assert.Len(t, pools, 1)
	assert.NotContains(t, pools, "ollama")
	assert.Contains(t, pools, "anthropic")
}

// TestNewPoolsFromConfig_UnknownType_ReturnsError verifies that an unknown
// credential type aborts factory construction with a descriptive error
// (OI-06 schema validation).
func TestNewPoolsFromConfig_UnknownType_ReturnsError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Providers: map[string]config.ProviderConfig{
				"anthropic": {
					Credentials: []config.CredentialConfig{
						{Type: "totally_unknown_type", Path: "/tmp/x", KeyringRef: "x"},
					},
				},
			},
		},
	}

	_, err := credential.NewPoolsFromConfig(context.Background(), cfg, zap.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "totally_unknown_type")
}

// TestNewPoolsFromConfig_MissingFile_TolerantPerOI05 verifies that a
// configured-but-absent vendor file is tolerated (the source returns an
// empty list per OI-05 rule 3); the resulting pool simply has zero entries.
func TestNewPoolsFromConfig_MissingFile_TolerantPerOI05(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Providers: map[string]config.ProviderConfig{
				"anthropic": {
					Credentials: []config.CredentialConfig{
						{Type: "anthropic_claude_file", Path: filepath.Join(t.TempDir(), "missing.json"), KeyringRef: "anth-missing"},
					},
				},
			},
		},
	}

	pools, err := credential.NewPoolsFromConfig(context.Background(), cfg, zap.NewNop())
	require.NoError(t, err)
	require.Len(t, pools, 1)
	pool := pools["anthropic"]
	require.NotNil(t, pool)
	total, _ := pool.Size()
	assert.Equal(t, 0, total)
}

// TestNewPoolsFromConfig_CancelledContext_PropagatesCtxErr verifies that a
// pre-cancelled context aborts factory construction.
func TestNewPoolsFromConfig_CancelledContext_PropagatesCtxErr(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Providers: map[string]config.ProviderConfig{
				"anthropic": {
					Credentials: []config.CredentialConfig{
						{Type: "anthropic_claude_file", Path: makeAnthropicFile(t), KeyringRef: "anth-1"},
					},
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := credential.NewPoolsFromConfig(ctx, cfg, zap.NewNop())
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestNewPoolsFromConfig_NilLogger_StillWorks verifies that the factory
// tolerates a nil logger (defensive default to zap.NewNop internally).
func TestNewPoolsFromConfig_NilLogger_StillWorks(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Providers: map[string]config.ProviderConfig{
				"openai": {
					Credentials: []config.CredentialConfig{
						{Type: "openai_codex_file", Path: makeOpenAIFile(t), KeyringRef: "oai-1"},
					},
				},
			},
		},
	}

	pools, err := credential.NewPoolsFromConfig(context.Background(), cfg, nil)
	require.NoError(t, err)
	require.Len(t, pools, 1)
}

// TestNewPoolsFromConfig_NilConfig_ReturnsError verifies the defensive nil
// check on the factory's required cfg argument.
func TestNewPoolsFromConfig_NilConfig_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := credential.NewPoolsFromConfig(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil config")
}

// TestNewPoolsFromConfig_EmptyPathUsesDefault verifies that an empty Path
// triggers default-path resolution (the source itself returns empty when
// the default path file doesn't exist on this machine, which is fine — we
// only need to confirm the factory does not error out).
func TestNewPoolsFromConfig_EmptyPathUsesDefault(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		LLM: config.LLMConfig{
			Providers: map[string]config.ProviderConfig{
				"anthropic": {
					Credentials: []config.CredentialConfig{
						{Type: "anthropic_claude_file", KeyringRef: "anth-default"}, // empty Path
					},
				},
				"openai": {
					Credentials: []config.CredentialConfig{
						{Type: "openai_codex_file", KeyringRef: "oai-default"}, // empty Path
					},
				},
				"nous": {
					Credentials: []config.CredentialConfig{
						{Type: "nous_hermes_file", KeyringRef: "nous-default"}, // empty Path
					},
				},
			},
		},
	}

	pools, err := credential.NewPoolsFromConfig(context.Background(), cfg, nil)
	require.NoError(t, err)
	assert.Len(t, pools, 3)
}
