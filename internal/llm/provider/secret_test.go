package provider_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// google genai SDK imports go.opencensus.io which starts a background goroutine.
	// This is a known false-positive; ignore it so matrix tests can import the google adapter.
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
	)
}

// TestFileSecretStore_Resolve_ReadsAccessToken은 JSON 파일에서 access_token을 읽는지 검증한다.
func TestFileSecretStore_Resolve_ReadsAccessToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	keyringID := "test-cred"
	credFile := filepath.Join(dir, keyringID+".json")

	data := map[string]any{"access_token": "tok-abc123", "expires_in": 3600}
	raw, _ := json.Marshal(data)
	require.NoError(t, os.WriteFile(credFile, raw, 0600))

	store := provider.NewFileSecretStore(dir)
	token, err := store.Resolve(context.Background(), keyringID)

	require.NoError(t, err)
	assert.Equal(t, "tok-abc123", token)
}

// TestFileSecretStore_Resolve_FileNotFound는 파일이 없을 때 에러를 반환하는지 검증한다.
func TestFileSecretStore_Resolve_FileNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := provider.NewFileSecretStore(dir)

	_, err := store.Resolve(context.Background(), "nonexistent")
	assert.Error(t, err)
}

// TestFileSecretStore_Resolve_PathTraversal은 ".." 포함 keyringID를 거부하는지 검증한다.
func TestFileSecretStore_Resolve_PathTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := provider.NewFileSecretStore(dir)

	_, err := store.Resolve(context.Background(), "../etc/passwd")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

// TestFileSecretStore_WriteBack_PersistsToken은 WriteBack이 access_token을 기록하는지 검증한다.
func TestFileSecretStore_WriteBack_PersistsToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	keyringID := "test-writeback"
	store := provider.NewFileSecretStore(dir)

	err := store.WriteBack(context.Background(), keyringID, "new-token-xyz")
	require.NoError(t, err)

	// 읽어서 확인
	token, err := store.Resolve(context.Background(), keyringID)
	require.NoError(t, err)
	assert.Equal(t, "new-token-xyz", token)
}

// TestFileSecretStore_WriteBack_PathTraversal은 ".." 포함 keyringID를 거부하는지 검증한다.
func TestFileSecretStore_WriteBack_PathTraversal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := provider.NewFileSecretStore(dir)

	err := store.WriteBack(context.Background(), "../../evil", "tok")
	assert.Error(t, err)
}

// TestFileSecretStore_WriteBack_Atomic은 atomic write가 작동하는지 검증한다.
// 파일 권한이 0600이어야 한다.
func TestFileSecretStore_WriteBack_FilePermission(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	keyringID := "perm-test"
	store := provider.NewFileSecretStore(dir)

	require.NoError(t, store.WriteBack(context.Background(), keyringID, "secret"))

	info, err := os.Stat(filepath.Join(dir, keyringID+".json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
