// Package credential_test covers the OpenAICodexSource implementation
// for SPEC-GOOSE-CREDPOOL-001 OI-05 / AC-CREDPOOL-007.
package credential_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOpenAICodexSource_MissingFile_ReturnsEmptyNoError verifies graceful
// handling of a missing ~/.codex/auth.json file (OI-05 rule 3).
func TestOpenAICodexSource_MissingFile_ReturnsEmptyNoError(t *testing.T) {
	t.Parallel()

	src := credential.NewOpenAICodexSource(filepath.Join(t.TempDir(), "missing.json"))
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, creds)
}

// TestOpenAICodexSource_OAuthTokens_EmitsMetadataOnly verifies that
// the tokens block is parsed into a metadata-only entry without leaking
// any of the OAuth token strings.
func TestOpenAICodexSource_OAuthTokens_EmitsMetadataOnly(t *testing.T) {
	t.Parallel()

	const accessToken = "openai-access-LEAK-CHECK"
	const refreshToken = "openai-refresh-LEAK-CHECK"
	const idToken = "openai-id-LEAK-CHECK"
	lastRefresh := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)

	body := `{
  "OPENAI_API_KEY": null,
  "tokens": {
    "access_token": "` + accessToken + `",
    "refresh_token": "` + refreshToken + `",
    "id_token": "` + idToken + `"
  },
  "last_refresh": "` + lastRefresh + `"
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, body)

	src := credential.NewOpenAICodexSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, creds, 1)

	c := creds[0]
	assert.NotEmpty(t, c.ID)
	assert.Equal(t, "openai", c.Provider)
	assert.NotEmpty(t, c.KeyringID)
	assert.Equal(t, credential.CredOK, c.Status)

	assertNoSecretLeak(t, c, accessToken, refreshToken, idToken)
}

// TestOpenAICodexSource_APIKeyOnly_HasZeroExpiresAt verifies that
// when only OPENAI_API_KEY is present (no OAuth tokens block), the entry
// is still emitted with ExpiresAt zero (permanent validity per
// REQ-CREDPOOL-001 (b)).
func TestOpenAICodexSource_APIKeyOnly_HasZeroExpiresAt(t *testing.T) {
	t.Parallel()

	body := `{"OPENAI_API_KEY": "sk-openai-LEAK-CHECK"}`

	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, body)

	src := credential.NewOpenAICodexSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, creds, 1)

	c := creds[0]
	assert.True(t, c.ExpiresAt.IsZero(),
		"API key should have zero ExpiresAt to signal permanent validity")
	assert.Equal(t, credential.CredOK, c.Status)
	assertNoSecretLeak(t, c, "sk-openai-LEAK-CHECK")
}

// TestOpenAICodexSource_NoCredentialMaterial_ReturnsEmpty verifies that
// a syntactically valid file with neither tokens nor api key yields an
// empty result with no error.
func TestOpenAICodexSource_NoCredentialMaterial_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	body := `{"OPENAI_API_KEY": null}`

	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, body)

	src := credential.NewOpenAICodexSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, creds)
}

// TestOpenAICodexSource_MalformedJSON_WrapsError verifies that
// invalid JSON yields a wrapped error containing the file path.
func TestOpenAICodexSource_MalformedJSON_WrapsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, `{"tokens":`)

	src := credential.NewOpenAICodexSource(path)
	_, err := src.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai")
	assert.Contains(t, err.Error(), path)
}

// TestOpenAICodexSource_CancelledContext_ReturnsCtxErr verifies REQ-CREDPOOL-017.
func TestOpenAICodexSource_CancelledContext_ReturnsCtxErr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, `{"OPENAI_API_KEY":"x"}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := credential.NewOpenAICodexSource(path).Load(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestDefaultOpenAICodexCredentialsPath returns a path ending with the
// expected vendor filename (best-effort smoke test).
func TestDefaultOpenAICodexCredentialsPath(t *testing.T) {
	t.Parallel()

	p := credential.DefaultOpenAICodexCredentialsPath()
	require.NotEmpty(t, p)
	assert.Contains(t, p, ".codex")
	assert.Contains(t, p, "auth.json")
}
