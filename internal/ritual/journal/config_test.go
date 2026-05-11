package journal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadJournalConfig_EmptyPath verifies that empty path returns privacy-safe defaults.
func TestLoadJournalConfig_EmptyPath(t *testing.T) {
	t.Parallel()

	cfg, err := LoadJournalConfig("")
	require.NoError(t, err)
	assert.False(t, cfg.Enabled, "default must be disabled (opt-in)")
	assert.False(t, cfg.AllowLoRATraining, "default must disable LoRA training")
	assert.Equal(t, -1, cfg.RetentionDays, "default retention must be -1 (keep forever)")
	assert.Equal(t, 60, cfg.PromptTimeoutMin, "default timeout must be 60 minutes")
}

// TestLoadJournalConfig_MissingFile verifies that a non-existent file returns defaults.
func TestLoadJournalConfig_MissingFile(t *testing.T) {
	t.Parallel()

	cfg, err := LoadJournalConfig("/nonexistent/path/journal.yaml")
	require.NoError(t, err, "missing file must not return error")
	assert.False(t, cfg.Enabled, "missing file must return disabled default")
}

// TestLoadJournalConfig_ParsesEnabled verifies that a valid YAML file is loaded.
func TestLoadJournalConfig_ParsesEnabled(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "journal.yaml")
	content := []byte("enabled: true\nretention_days: 30\nprompt_timeout_min: 5\n")
	require.NoError(t, os.WriteFile(path, content, 0600))

	cfg, err := LoadJournalConfig(path)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30, cfg.RetentionDays)
	assert.Equal(t, 5, cfg.PromptTimeoutMin)
}

// TestLoadJournalConfig_PartialYAML_DefaultsPreserved verifies that fields absent from
// the YAML file retain their privacy-safe defaults.
func TestLoadJournalConfig_PartialYAML_DefaultsPreserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "journal.yaml")
	// Only set enabled; retention_days and prompt_timeout_min are absent.
	require.NoError(t, os.WriteFile(path, []byte("enabled: true\n"), 0600))

	cfg, err := LoadJournalConfig(path)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, -1, cfg.RetentionDays, "absent retention_days must default to -1")
	assert.Equal(t, 60, cfg.PromptTimeoutMin, "absent prompt_timeout_min must default to 60")
}
