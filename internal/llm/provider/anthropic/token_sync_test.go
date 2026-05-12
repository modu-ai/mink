package anthropic_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/llm/provider/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadClaudeCredentials는 credentials 파일을 올바르게 읽는지 검증한다.
func TestReadClaudeCredentials(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	credFile := filepath.Join(dir, ".credentials.json")

	expiresAt := time.Now().Add(3600 * time.Second).UTC().Truncate(time.Second)
	data := map[string]any{
		"access_token":  "tok-abc",
		"refresh_token": "ref-xyz",
		"expires_at":    expiresAt.Format(time.RFC3339),
		"client_id":     "client-123",
	}

	raw := anthropic.MarshalJSON(data)
	require.NoError(t, os.WriteFile(credFile, raw, 0600))

	creds, err := anthropic.ReadClaudeCredentials(credFile)
	require.NoError(t, err)
	assert.Equal(t, "tok-abc", creds.AccessToken)
	assert.Equal(t, "ref-xyz", creds.RefreshToken)
	assert.Equal(t, "client-123", creds.ClientID)
}

// TestAtomicWriteClaudeCredentials는 atomic write가 작동하는지 검증한다.
func TestAtomicWriteClaudeCredentials(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	credFile := filepath.Join(dir, ".credentials.json")

	creds := &anthropic.ClaudeCredentials{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Now().Add(3600 * time.Second),
		ClientID:     "my-client",
	}

	err := anthropic.AtomicWriteClaudeCredentials(credFile, creds)
	require.NoError(t, err)

	// 파일 권한 확인
	info, err := os.Stat(credFile)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// 내용 검증
	readBack, err := anthropic.ReadClaudeCredentials(credFile)
	require.NoError(t, err)
	assert.Equal(t, "new-access", readBack.AccessToken)
	assert.Equal(t, "new-refresh", readBack.RefreshToken)
}

// TestAtomicWriteClaudeCredentials_NoTmpFileLeft는 write 후 임시 파일이 없는지 검증한다.
func TestAtomicWriteClaudeCredentials_NoTmpFileLeft(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	credFile := filepath.Join(dir, ".credentials.json")

	creds := &anthropic.ClaudeCredentials{
		AccessToken: "tok",
	}

	require.NoError(t, anthropic.AtomicWriteClaudeCredentials(credFile, creds))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	for _, entry := range entries {
		assert.False(t, len(entry.Name()) > len(".credentials.json"),
			"임시 파일이 남아 있으면 안 됨: %s", entry.Name())
	}
}
