package qmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds QMD subsystem configuration.
// @MX:NOTE: Config is the single source of truth for QMD behavior
type Config struct {
	// Enabled controls whether QMD is active.
	Enabled bool

	// IndexPath is the directory where index data is stored.
	IndexPath string

	// ModelsPath is the directory where GGUF models are stored.
	ModelsPath string

	// EmbedderModel is the filename of the embedding model.
	EmbedderModel string

	// RerankerModel is the filename of the reranker model.
	RerankerModel string

	// MaxResults is the default maximum number of search results.
	MaxResults int

	// MemoryLimitMB is the maximum memory limit for the index (in MB).
	MemoryLimitMB int

	// WatcherEnabled enables automatic file watching.
	WatcherEnabled bool

	// WatchDebounceMs is the debounce delay for file events in milliseconds.
	WatchDebounceMs int

	// MCPEnabled enables the MCP stdio server.
	MCPEnabled bool

	// IndexRoots is the list of root paths to index.
	IndexRoots []string

	// ModelMirrorEnv is the environment variable name for custom model mirror.
	ModelMirrorEnv string
}

// DefaultConfig returns a configuration with sensible defaults.
// @MX:ANCHOR: Default configuration factory (expected fan_in >= 3)
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		IndexPath:       "./.goose/data/qmd-index/",
		ModelsPath:      "./.goose/data/models/",
		EmbedderModel:   "bge-small-en-v1.5.gguf",
		RerankerModel:   "bge-reranker-base.gguf",
		MaxResults:      10,
		MemoryLimitMB:   500,
		WatcherEnabled:  false,
		WatchDebounceMs: 500,
		MCPEnabled:      false,
		IndexRoots: []string{
			"./.goose/memory",
			"./.goose/context",
			"./.goose/skills",
			"./.goose/tasks",
			"./.goose/rituals",
		},
		ModelMirrorEnv: "QMD_MODEL_MIRROR",
	}
}

// Validate checks if the configuration is valid.
// Returns an error if any required field is invalid.
func (c *Config) Validate() error {
	if c.IndexPath == "" {
		return fmt.Errorf("%w: IndexPath cannot be empty", ErrIndexPathInvalid)
	}

	if c.MaxResults <= 0 {
		return fmt.Errorf("%w: MaxResults must be positive", ErrIndexPathInvalid)
	}

	if c.MemoryLimitMB < 0 {
		return fmt.Errorf("%w: MemoryLimitMB cannot be negative", ErrIndexPathInvalid)
	}

	if c.WatchDebounceMs < 0 {
		return fmt.Errorf("%w: WatchDebounceMs cannot be negative", ErrIndexPathInvalid)
	}

	return nil
}

// EnsurePaths creates the index and model directories if they don't exist.
func (c *Config) EnsurePaths() error {
	dirs := []string{
		c.IndexPath,
		c.ModelsPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// GetIndexPath returns the absolute path to the index directory.
func (c *Config) GetIndexPath() (string, error) {
	if c.IndexPath == "" {
		return "", ErrIndexPathInvalid
	}
	return filepath.Abs(c.IndexPath)
}

// GetModelsPath returns the absolute path to the models directory.
func (c *Config) GetModelsPath() (string, error) {
	if c.ModelsPath == "" {
		return "", ErrModelNotAvailable
	}
	return filepath.Abs(c.ModelsPath)
}

// GetEmbedderModelPath returns the absolute path to the embedder model file.
func (c *Config) GetEmbedderModelPath() (string, error) {
	modelsDir, err := c.GetModelsPath()
	if err != nil {
		return "", err
	}

	modelPath := filepath.Join(modelsDir, c.EmbedderModel)
	if _, err := os.Stat(modelPath); errors.Is(err, os.ErrNotExist) {
		return "", ErrModelNotAvailable
	}

	return filepath.Abs(modelPath)
}

// GetRerankerModelPath returns the absolute path to the reranker model file.
func (c *Config) GetRerankerModelPath() (string, error) {
	modelsDir, err := c.GetModelsPath()
	if err != nil {
		return "", err
	}

	modelPath := filepath.Join(modelsDir, c.RerankerModel)
	if _, err := os.Stat(modelPath); errors.Is(err, os.ErrNotExist) {
		return "", ErrModelNotAvailable
	}

	return filepath.Abs(modelPath)
}
