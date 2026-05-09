package telegram_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/modu-ai/goose/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeConfig writes content to a temp file and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "telegram.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// TestLoadConfig_Valid verifies that a valid YAML file without bot_token is loaded correctly.
func TestLoadConfig_Valid(t *testing.T) {
	path := writeConfig(t, `
bot_username: testbot
allowed_users: [111, 222]
mode: polling
audit_enabled: true
auto_admit_first_user: false
default_streaming: false
`)

	cfg, err := telegram.LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "testbot", cfg.BotUsername)
	assert.Equal(t, []int64{111, 222}, cfg.AllowedUsers)
	assert.Equal(t, "polling", cfg.Mode)
	assert.True(t, cfg.AuditEnabled)
	assert.False(t, cfg.AutoAdmitFirstUser)
	assert.False(t, cfg.DefaultStreaming)
}

// TestLoadConfig_TokenRejected verifies that a yaml with bot_token: non-empty is rejected.
func TestLoadConfig_TokenRejected(t *testing.T) {
	path := writeConfig(t, `
bot_username: testbot
bot_token: "abc:123"
`)

	_, err := telegram.LoadConfig(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, telegram.ErrPlainTokenRejected)
}

// TestLoadConfig_EmptyTokenRejected verifies that even an empty bot_token field is rejected.
func TestLoadConfig_EmptyTokenRejected(t *testing.T) {
	path := writeConfig(t, `
bot_username: testbot
bot_token: ""
`)

	_, err := telegram.LoadConfig(path)
	require.Error(t, err)
	assert.ErrorIs(t, err, telegram.ErrPlainTokenRejected)
}

// TestLoadConfig_MissingFile verifies that a missing file wraps fs.ErrNotExist.
func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := telegram.LoadConfig("/nonexistent/path/telegram.yaml")
	require.Error(t, err)
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

// TestLoadConfig_InvalidYAML verifies that malformed YAML returns an error.
func TestLoadConfig_InvalidYAML(t *testing.T) {
	// Use a YAML mapping with an unclosed bracket, which the parser rejects.
	path := writeConfig(t, "allowed_users: [1, 2")
	_, err := telegram.LoadConfig(path)
	require.Error(t, err)
}

// TestLoadConfig_DefaultMode verifies that mode defaults to "polling" when omitted.
func TestLoadConfig_DefaultMode(t *testing.T) {
	path := writeConfig(t, `
bot_username: testbot
`)

	cfg, err := telegram.LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "polling", cfg.Mode)
}

// TestLoadConfig_EmptyFile verifies that an empty YAML file yields a default Config.
func TestLoadConfig_EmptyFile(t *testing.T) {
	path := writeConfig(t, "")
	cfg, err := telegram.LoadConfig(path)
	require.NoError(t, err)
	// Default mode should be set even for empty config
	assert.Equal(t, "polling", cfg.Mode)
}
