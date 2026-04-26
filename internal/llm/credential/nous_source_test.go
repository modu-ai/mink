// Package credential_test covers the NousSource implementation
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

// TestNousSource_MissingFile_ReturnsEmptyNoError verifies graceful handling
// of a missing ~/.hermes/auth.json (OI-05 rule 3).
func TestNousSource_MissingFile_ReturnsEmptyNoError(t *testing.T) {
	t.Parallel()

	src := credential.NewNousSource(filepath.Join(t.TempDir(), "missing.json"))
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, creds)
}

// TestNousSource_ValidFile_EmitsMetadataOnly verifies that the agent_key is
// not leaked into PooledCredential's metadata fields.
func TestNousSource_ValidFile_EmitsMetadataOnly(t *testing.T) {
	t.Parallel()

	const agentKey = "hermes-agent-key-LEAK-CHECK"
	expires := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)

	body := `{
  "agent_key": "` + agentKey + `",
  "expires_at": "` + expires.Format(time.RFC3339) + `"
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, body)

	src := credential.NewNousSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, creds, 1)

	c := creds[0]
	assert.NotEmpty(t, c.ID)
	assert.Equal(t, "nous", c.Provider)
	assert.NotEmpty(t, c.KeyringID)
	assert.Equal(t, credential.CredOK, c.Status)
	assert.WithinDuration(t, expires, c.ExpiresAt, time.Second)

	assertNoSecretLeak(t, c, agentKey)
}

// TestNousSource_NoExpiry_HasZeroExpiresAt verifies that an absent expires_at
// field results in a permanently valid entry (ExpiresAt zero).
func TestNousSource_NoExpiry_HasZeroExpiresAt(t *testing.T) {
	t.Parallel()

	body := `{"agent_key": "k"}`
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, body)

	src := credential.NewNousSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, creds, 1)
	assert.True(t, creds[0].ExpiresAt.IsZero())
}

// TestNousSource_ExpiredKey_StatusExhausted verifies that a past expires_at
// emits Status=CredExhausted.
func TestNousSource_ExpiredKey_StatusExhausted(t *testing.T) {
	t.Parallel()

	body := `{
  "agent_key": "k",
  "expires_at": "` + time.Now().Add(-1*time.Hour).UTC().Format(time.RFC3339) + `"
}`
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, body)

	src := credential.NewNousSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, creds, 1)
	assert.Equal(t, credential.CredExhausted, creds[0].Status)
}

// TestNousSource_NoAgentKey_ReturnsEmpty verifies that a syntactically valid
// file without agent_key produces an empty result with no error.
func TestNousSource_NoAgentKey_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	body := `{"expires_at": "2030-01-01T00:00:00Z"}`
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, body)

	src := credential.NewNousSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, creds)
}

// TestNousSource_MalformedJSON_WrapsError verifies that invalid JSON yields a
// wrapped error containing the file path.
func TestNousSource_MalformedJSON_WrapsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, `{"agent_key":`)

	src := credential.NewNousSource(path)
	_, err := src.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nous")
	assert.Contains(t, err.Error(), path)
}

// TestNousSource_CancelledContext_ReturnsCtxErr verifies REQ-CREDPOOL-017.
func TestNousSource_CancelledContext_ReturnsCtxErr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, `{"agent_key":"k"}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := credential.NewNousSource(path).Load(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestDefaultNousCredentialsPath returns a path ending with the expected
// vendor filename (best-effort smoke test).
func TestDefaultNousCredentialsPath(t *testing.T) {
	t.Parallel()

	p := credential.DefaultNousCredentialsPath()
	require.NotEmpty(t, p)
	assert.Contains(t, p, ".hermes")
	assert.Contains(t, p, "auth.json")
}

// TestNousSource_MalformedExpiresAt_TolerantNoError verifies that an invalid
// RFC3339 expires_at value falls back to "no expiry" (permanent validity)
// rather than aborting the entire load.
func TestNousSource_MalformedExpiresAt_TolerantNoError(t *testing.T) {
	t.Parallel()

	body := `{"agent_key":"k","expires_at":"not-a-date"}`
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	writeFile(t, path, body)

	src := credential.NewNousSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, creds, 1)
	assert.True(t, creds[0].ExpiresAt.IsZero())
}
