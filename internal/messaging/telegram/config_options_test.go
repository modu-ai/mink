package telegram

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig_SilentDefault verifies that silent_default: true is parsed
// from yaml and stored in Config.SilentDefault (REQ-MTGM-O01).
func TestLoadConfig_SilentDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telegram.yaml")
	content := `
bot_username: testbot
allowed_users: [123]
mode: polling
silent_default: true
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.SilentDefault {
		t.Errorf("SilentDefault: got false, want true")
	}
}

// TestLoadConfig_TypingIndicator verifies that typing_indicator: true is parsed
// from yaml and stored in Config.TypingIndicator (REQ-MTGM-O02).
func TestLoadConfig_TypingIndicator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telegram.yaml")
	content := `
bot_username: testbot
allowed_users: [123]
mode: polling
typing_indicator: true
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.TypingIndicator {
		t.Errorf("TypingIndicator: got false, want true")
	}
}

// TestLoadConfig_SilentDefault_Omitted verifies that omitting silent_default
// defaults to false (must not break existing configs).
func TestLoadConfig_SilentDefault_Omitted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "telegram.yaml")
	content := `
bot_username: testbot
allowed_users: [123]
mode: polling
`
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.SilentDefault {
		t.Errorf("SilentDefault: got true, want false (default)")
	}
	if cfg.TypingIndicator {
		t.Errorf("TypingIndicator: got true, want false (default)")
	}
}
