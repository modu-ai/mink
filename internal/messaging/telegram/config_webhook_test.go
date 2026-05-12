package telegram_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/modu-ai/mink/internal/messaging/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeYAML writes content to a temp file and returns its path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "telegram-*.yaml")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// TestLoadConfig_Webhook_Defaults verifies that when webhook section is absent
// FallbackToPolling defaults to true.
func TestLoadConfig_Webhook_Defaults(t *testing.T) {
	path := writeYAML(t, `bot_username: testbot
mode: webhook
`)
	cfg, err := telegram.LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "webhook", cfg.Mode)
	// FallbackToPolling must default to true when not specified.
	assert.True(t, cfg.Webhook.FallbackToPolling)
	assert.Empty(t, cfg.Webhook.PublicURL)
	assert.Empty(t, cfg.Webhook.Secret)
	assert.Empty(t, cfg.Webhook.ListenAddr)
}

// TestLoadConfig_Webhook_ExplicitFalse verifies that FallbackToPolling: false
// is correctly preserved.
func TestLoadConfig_Webhook_ExplicitFalse(t *testing.T) {
	path := writeYAML(t, `bot_username: testbot
mode: webhook
webhook:
  public_url: "https://bot.example.com"
  secret: "abc123"
  listen_addr: ":8443"
  fallback_to_polling: false
`)
	cfg, err := telegram.LoadConfig(path)
	require.NoError(t, err)
	assert.False(t, cfg.Webhook.FallbackToPolling)
	assert.Equal(t, "https://bot.example.com", cfg.Webhook.PublicURL)
	assert.Equal(t, "abc123", cfg.Webhook.Secret)
	assert.Equal(t, ":8443", cfg.Webhook.ListenAddr)
}

// TestLoadConfig_Webhook_ExplicitTrue verifies that FallbackToPolling: true
// is correctly preserved.
func TestLoadConfig_Webhook_ExplicitTrue(t *testing.T) {
	path := writeYAML(t, `bot_username: testbot
mode: webhook
webhook:
  public_url: "https://bot.example.com"
  fallback_to_polling: true
`)
	cfg, err := telegram.LoadConfig(path)
	require.NoError(t, err)
	assert.True(t, cfg.Webhook.FallbackToPolling)
}

// TestLoadConfig_Webhook_PollingModeNoWebhookSection verifies that a
// polling-mode config without webhook section is loaded cleanly with
// FallbackToPolling defaulting to true.
func TestLoadConfig_Webhook_PollingModeNoWebhookSection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "telegram.yaml")
	require.NoError(t, os.WriteFile(path, []byte("bot_username: bot\n"), 0o600))

	cfg, err := telegram.LoadConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "polling", cfg.Mode)
	assert.True(t, cfg.Webhook.FallbackToPolling)
}
