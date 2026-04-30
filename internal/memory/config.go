package memory

import (
	"fmt"
	"os"
	"path/filepath"
)

// MemoryConfig holds configuration for the memory system.
type MemoryConfig struct {
	Builtin BuiltinConfig `yaml:"builtin"`
	Plugin  PluginConfig  `yaml:"plugin"`
}

// BuiltinConfig holds configuration for the BuiltinProvider.
type BuiltinConfig struct {
	// DBPath is the path to the SQLite database file.
	// Defaults to ~/.goose/memory/memory.db
	DBPath string `yaml:"db_path"`

	// MaxRows is the maximum number of facts to store in the facts table.
	// When exceeded, oldest facts are evicted (FIFO).
	// Defaults to 10000.
	MaxRows int `yaml:"max_rows"`
}

// PluginConfig holds configuration for external plugin providers.
type PluginConfig struct {
	// Name is the import path of the plugin (e.g., "github.com/goose/memory-honcho").
	// Empty string means no plugin is configured.
	Name string `yaml:"name"`

	// Config is plugin-specific configuration (arbitrary YAML).
	// The plugin factory is responsible for parsing this.
	Config map[string]any `yaml:"config"`
}

// ApplyDefaults sets sensible defaults for unset configuration values.
func (c *MemoryConfig) ApplyDefaults() {
	if c.Builtin.DBPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			c.Builtin.DBPath = filepath.Join(homeDir, ".goose", "memory", "memory.db")
		} else {
			// Fallback if home directory cannot be determined
			c.Builtin.DBPath = "memory.db"
		}
	}

	if c.Builtin.MaxRows == 0 {
		c.Builtin.MaxRows = 10000
	}
}

// Validate checks that the configuration is valid.
func (c *MemoryConfig) Validate() error {
	// Builtin validation
	if c.Builtin.MaxRows < 1 {
		return fmt.Errorf("builtin.max_rows must be >= 1, got %d", c.Builtin.MaxRows)
	}

	// Plugin validation - empty name is valid (no plugin)
	// Non-empty names will be validated by the plugin registry

	return nil
}

// DefaultMemoryConfig returns a MemoryConfig with all defaults applied.
func DefaultMemoryConfig() MemoryConfig {
	var cfg MemoryConfig
	cfg.ApplyDefaults()
	return cfg
}
