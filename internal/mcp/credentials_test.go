package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── T-011: MCP credentials userpath 마이그레이션 ──────────────────────────────

// TestCredentialPath_UsesMinkDir는 credentialPath가 ~/.mink/mcp-credentials/ 를
// 사용함을 검증한다.
// REQ-MINK-UDM-002. AC-005.
func TestCredentialPath_UsesMinkDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })

	path, err := credentialPath("test-server")
	require.NoError(t, err)

	expected := filepath.Join(fakeHome, ".mink", "mcp-credentials", "test-server.json")
	assert.Equal(t, expected, path, "credential path must use .mink, REQ-MINK-UDM-002")
}

// TestSaveLoadCredential_MinkPath는 SaveCredential + LoadCredential 이
// .mink 경로에 저장/로드됨을 검증한다.
func TestSaveLoadCredential_MinkPath(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	os.Unsetenv("MINK_HOME")
	t.Cleanup(func() { os.Unsetenv("MINK_HOME") })

	ts := &TokenSet{
		AccessToken:  "tok123",
		RefreshToken: "ref456",
		Scope:        "read write",
	}

	err := SaveCredential("my-server", ts)
	require.NoError(t, err)

	// 파일이 .mink/mcp-credentials/ 에 존재해야 함
	expected := filepath.Join(fakeHome, ".mink", "mcp-credentials", "my-server.json")
	_, statErr := os.Stat(expected)
	assert.NoError(t, statErr, "credential file must exist under .mink")

	loaded, err := LoadCredential("my-server", nil)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, "tok123", loaded.AccessToken)
	assert.Equal(t, "ref456", loaded.RefreshToken)
}
