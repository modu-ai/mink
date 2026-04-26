// Package credential_test covers the AnthropicClaudeSource implementation
// for SPEC-GOOSE-CREDPOOL-001 OI-05 / AC-CREDPOOL-007.
package credential_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeFile writes content to the given path with mode 0600 (vendor convention).
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// TestAnthropicClaudeSource_MissingFile_ReturnsEmptyNoError verifies that
// an absent ~/.claude/.credentials.json yields an empty slice and no error
// (per OI-05 rule 3 and the broader Source contract used by DummySource).
func TestAnthropicClaudeSource_MissingFile_ReturnsEmptyNoError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	src := credential.NewAnthropicClaudeSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, creds)
}

// TestAnthropicClaudeSource_ValidFile_EmitsMetadataOnly verifies that
// the parsed result contains only metadata: ID, Provider, KeyringID, Status,
// ExpiresAt. Raw token fields must NEVER appear in the returned struct
// (REQ-CREDPOOL-014 enforced at struct level via TestPooledCredential_NoSecretFields).
// Here we additionally assert that no secret-string from the source file leaks
// into PooledCredential's exported fields.
func TestAnthropicClaudeSource_ValidFile_EmitsMetadataOnly(t *testing.T) {
	t.Parallel()

	const accessToken = "sk-anthro-secret-DO-NOT-LEAK"
	const refreshToken = "refresh-token-DO-NOT-LEAK"
	expires := time.Now().Add(2 * time.Hour)
	expiresMs := expires.UnixMilli()

	body := `{
  "claudeAiOauth": {
    "accessToken": "` + accessToken + `",
    "refreshToken": "` + refreshToken + `",
    "expiresAt": ` + itoa64(expiresMs) + `
  }
}`

	dir := t.TempDir()
	path := filepath.Join(dir, ".credentials.json")
	writeFile(t, path, body)

	src := credential.NewAnthropicClaudeSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, creds, 1)

	c := creds[0]
	assert.NotEmpty(t, c.ID)
	assert.Equal(t, "anthropic", c.Provider)
	assert.NotEmpty(t, c.KeyringID, "KeyringID must be a non-empty reference label")
	assert.Equal(t, credential.CredOK, c.Status)
	assert.WithinDuration(t, expires, c.ExpiresAt, time.Second)

	// No raw secret may appear in any string field of the metadata-only entry.
	assertNoSecretLeak(t, c, accessToken, refreshToken)
}

// TestAnthropicClaudeSource_ExpiredToken_StatusExhausted verifies that
// an entry whose expiresAt is already in the past is emitted with
// Status=CredExhausted (so the pool's available filter excludes it without
// requiring a separate filtering pass in the Source).
func TestAnthropicClaudeSource_ExpiredToken_StatusExhausted(t *testing.T) {
	t.Parallel()

	pastMs := time.Now().Add(-1 * time.Hour).UnixMilli()
	body := `{
  "claudeAiOauth": {
    "accessToken": "x",
    "refreshToken": "y",
    "expiresAt": ` + itoa64(pastMs) + `
  }
}`

	dir := t.TempDir()
	path := filepath.Join(dir, ".credentials.json")
	writeFile(t, path, body)

	src := credential.NewAnthropicClaudeSource(path)
	creds, err := src.Load(context.Background())
	require.NoError(t, err)
	require.Len(t, creds, 1)
	assert.Equal(t, credential.CredExhausted, creds[0].Status)
}

// TestAnthropicClaudeSource_MalformedJSON_WrapsError verifies that
// invalid JSON yields a wrapped error referencing the file path.
func TestAnthropicClaudeSource_MalformedJSON_WrapsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".credentials.json")
	writeFile(t, path, "{ this is : not valid json")

	src := credential.NewAnthropicClaudeSource(path)
	_, err := src.Load(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "anthropic", "error must mention provider context")
	assert.Contains(t, err.Error(), path, "error must mention file path")
}

// TestAnthropicClaudeSource_CancelledContext_ReturnsCtxErr verifies that
// a pre-cancelled context aborts the load (REQ-CREDPOOL-017).
func TestAnthropicClaudeSource_CancelledContext_ReturnsCtxErr(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, ".credentials.json")
	writeFile(t, path, `{"claudeAiOauth":{"accessToken":"x","refreshToken":"y","expiresAt":1}}`)

	src := credential.NewAnthropicClaudeSource(path)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := src.Load(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestDefaultAnthropicClaudeCredentialsPath returns a path under the user home
// (best-effort smoke test; we only assert it is non-empty and ends with the
// expected vendor file name, since the actual home dir varies per environment).
func TestDefaultAnthropicClaudeCredentialsPath(t *testing.T) {
	t.Parallel()

	p := credential.DefaultAnthropicClaudeCredentialsPath()
	require.NotEmpty(t, p)
	assert.True(t, strings.HasSuffix(p, filepath.Join(".claude", ".credentials.json")),
		"default path should end with .claude/.credentials.json, got %q", p)
}

// assertNoSecretLeak walks every string-typed exported field of the credential
// and ensures none of the provided secret values appear. Defensive cross-check
// that complements the struct-level reflection invariant in
// TestPooledCredential_NoSecretFields.
func assertNoSecretLeak(t *testing.T, c *credential.PooledCredential, secrets ...string) {
	t.Helper()
	v := reflect.ValueOf(*c)
	tp := v.Type()
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() != reflect.String {
			continue
		}
		val := f.String()
		for _, s := range secrets {
			if s == "" {
				continue
			}
			assert.NotContains(t, val, s,
				"field %s leaked secret value %q", tp.Field(i).Name, s)
		}
	}
}

// itoa64 converts int64 to its decimal representation without importing strconv
// at the top of the test file solely for this helper.
func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
