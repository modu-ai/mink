package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestMemoryConfig_Defaults verifies that zero-value produces correct defaults.
func TestMemoryConfig_Defaults(t *testing.T) {
	var cfg MemoryConfig
	cfg.ApplyDefaults()

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expectedPath := filepath.Join(homeDir, ".goose", "memory", "memory.db")
	assert.Equal(t, expectedPath, cfg.Builtin.DBPath)
	assert.Equal(t, 10000, cfg.Builtin.MaxRows)
}

// TestMemoryConfig_YAMLDeserialization verifies YAML round-trip.
func TestMemoryConfig_YAMLDeserialization(t *testing.T) {
	yamlData := `
builtin:
  db_path: /custom/path/memory.db
  max_rows: 5000
plugin:
  name: github.com/goose/memory-honcho
  config:
    api_key: test-key
`

	var cfg MemoryConfig
	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	assert.Equal(t, "/custom/path/memory.db", cfg.Builtin.DBPath)
	assert.Equal(t, 5000, cfg.Builtin.MaxRows)
	assert.Equal(t, "github.com/goose/memory-honcho", cfg.Plugin.Name)
	assert.Equal(t, "test-key", cfg.Plugin.Config["api_key"])

	// Marshal back
	out, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.Contains(t, string(out), "db_path: /custom/path/memory.db")
}

// TestMemoryConfig_EmptyPluginName verifies that empty plugin name is valid.
func TestMemoryConfig_EmptyPluginName(t *testing.T) {
	cfg := MemoryConfig{
		Builtin: BuiltinConfig{
			DBPath:  "test.db",
			MaxRows: 100,
		},
		Plugin: PluginConfig{
			Name: "", // Empty is valid (no plugin)
		},
	}

	cfg.ApplyDefaults()
	err := cfg.Validate()
	assert.NoError(t, err)
}

// TestMemoryConfig_BuiltinDefaults verifies db_path and max_rows defaults.
func TestMemoryConfig_BuiltinDefaults(t *testing.T) {
	cfg := MemoryConfig{}
	cfg.ApplyDefaults()

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	expectedPath := filepath.Join(homeDir, ".goose", "memory", "memory.db")
	assert.Equal(t, expectedPath, cfg.Builtin.DBPath)
	assert.Equal(t, 10000, cfg.Builtin.MaxRows)
}

// TestBuiltinConfig_DefaultMaxRows verifies 10000 default.
func TestBuiltinConfig_DefaultMaxRows(t *testing.T) {
	cfg := MemoryConfig{}
	cfg.ApplyDefaults()

	assert.Equal(t, 10000, cfg.Builtin.MaxRows)
}

// TestMemoryConfig_Validate_MaxRowsTooLow verifies validation error for max_rows < 1.
func TestMemoryConfig_Validate_MaxRowsTooLow(t *testing.T) {
	cfg := MemoryConfig{
		Builtin: BuiltinConfig{
			MaxRows: 0,
		},
	}

	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max_rows must be >= 1")
}
