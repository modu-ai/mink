// Package aliasconfig provides an alias configuration file loader for model aliases.
// SPEC-GOOSE-ALIAS-CONFIG-001, SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001
package aliasconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/modu-ai/mink/internal/command"
	"github.com/modu-ai/mink/internal/llm/router"
	"go.uber.org/zap"
)

// Sentinel errors
var (
	// ErrConfigNotFound is returned when the aliases.yaml config file cannot be found.
	ErrConfigNotFound = errors.New("aliasconfig: config file not found")
)

// defaultConfigPath is the default config file path relative to the home directory.
const defaultConfigPath = ".goose/aliases.yaml"

// homeEnv is the environment variable key for the Mink home directory.
const homeEnv = "GOOSE_HOME"

// Logger is the logging interface used by Loader.
type Logger interface {
	Debug(string, ...zap.Field)
	Info(string, ...zap.Field)
	Warn(string, ...zap.Field)
	Error(string, ...zap.Field)
}

// Loader is the alias config loader.
// @MX:ANCHOR: [AUTO] Public API boundary; LoadDefault/Validate satisfy HOTRELOAD-001 §6.7 Loader/Validator interfaces
// @MX:REASON: fan_in >= 3 — consumed by ContextAdapter, Watcher (HOTRELOAD-001), and LoadEntries callers
// @MX:SPEC: SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-001/002
type Loader struct {
	configPath  string
	fsys        fs.FS
	logger      Logger
	stdLogger   *log.Logger
	mergePolicy MergePolicy
	metrics     Metrics
}

// Options holds construction options for Loader.
// Existing fields are preserved byte-identical; new fields use zero-value defaults
// to maintain backward compatibility.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001
type Options struct {
	// ConfigPath is the config file path. Empty string uses the default fallback chain.
	ConfigPath string
	// FS is the filesystem interface. Nil uses os-backed filesystem.
	FS fs.FS
	// Logger is the structured logger. Nil uses zap.NewNop().
	Logger Logger
	// StdLogger is the standard logger. Nil disables std logging.
	StdLogger *log.Logger

	// MergePolicy controls user/project file overlay behavior.
	// Zero value (MergePolicyProjectOverride) is the default and applies project-override semantics.
	//
	// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-020
	MergePolicy MergePolicy

	// Metrics receives observability events from Load/Reload/LoadDefault.
	// Nil uses an internal noop implementation — no allocation in steady state.
	//
	// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-022
	Metrics Metrics
}

// New creates a new Loader with the given options.
// Zero-value Options fields produce a Loader with default behavior
// identical to the v0.1.0 parent SPEC implementation.
func New(opts Options) *Loader {
	configPath := opts.ConfigPath
	if configPath == "" {
		// P3: Check project-local overlay first
		if projectLocalPath := detectProjectLocalAliasFile(); projectLocalPath != "" {
			configPath = projectLocalPath
		} else if home := os.Getenv(homeEnv); home != "" {
			configPath = filepath.Join(home, "aliases.yaml")
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				configPath = filepath.Join(homeDir, defaultConfigPath)
			} else {
				configPath = filepath.Join(homeDir, defaultConfigPath)
			}
		}
	}

	filesystem := opts.FS
	if filesystem == nil {
		// When no custom filesystem provided, use os-backed FS for full path support
		filesystem = &osFS{}
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	m := opts.Metrics
	if m == nil {
		m = noopMetrics{}
	}

	return &Loader{
		configPath:  configPath,
		fsys:        filesystem,
		logger:      logger,
		stdLogger:   opts.StdLogger,
		mergePolicy: opts.MergePolicy,
		metrics:     m,
	}
}

// ConfigPath returns the effective config path resolved at New(...) time,
// including any fallback chain result (project-local / GOOSE_HOME / HOME).
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-001
func (l *Loader) ConfigPath() string {
	return l.configPath
}

// Reload re-reads the alias file from disk with semantics identical to Load().
// Exported as a separate symbol so hot-reload call sites are grep-able and
// intent-aligned with HOTRELOAD-001's reload chain.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-002
func (l *Loader) Reload() (map[string]string, error) {
	return l.Load()
}

// AliasConfig is the alias configuration file structure (flat form).
// Extended form uses AliasEntry values — see entries.go.
type AliasConfig struct {
	// Aliases maps alias name → provider/model canonical string.
	Aliases map[string]string `yaml:"aliases"`
}

// maxAliasFileSize is the maximum allowed size for the alias file (1 MiB).
const maxAliasFileSize = 1 * 1024 * 1024

// detectProjectLocalAliasFile probes $CWD/.goose/aliases.yaml and returns
// its absolute path when it exists. Returns empty string otherwise.
// Supports project-local config overlay (P3 feature).
func detectProjectLocalAliasFile() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	projectLocalPath := filepath.Join(cwd, ".goose", "aliases.yaml")
	if _, err := os.Stat(projectLocalPath); err == nil {
		return projectLocalPath
	}

	return ""
}

// Load reads the alias config file and returns the parsed alias map.
// Uses the fs.FS interface for testability (P3 feature).
// Emits metrics via Options.Metrics when configured.
func (l *Loader) Load() (map[string]string, error) {
	start := time.Now()
	l.logger.Debug("loading alias config", zap.String("path", l.configPath))

	if l.stdLogger != nil {
		l.stdLogger.Printf("[aliasconfig] loading config from: %s", l.configPath)
	}

	// Check file size before reading to prevent large file parsing
	fileInfo, err := fs.Stat(l.fsys, l.configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			l.logger.Info("alias config not found, using empty map", zap.String("path", l.configPath))
			if l.stdLogger != nil {
				l.stdLogger.Printf("[aliasconfig] config file not found: %s", l.configPath)
			}
			l.metrics.IncLoadCount(true)
			l.metrics.RecordLoadDuration(time.Since(start))
			l.metrics.ObserveEntryCount(0)
			return nil, nil // File not found is not an error (return nil map)
		}
		l.metrics.IncLoadCount(false)
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}

	// Enforce 1 MiB size limit
	if fileInfo.Size() > maxAliasFileSize {
		if l.stdLogger != nil {
			l.stdLogger.Printf("[aliasconfig] file too large: %d bytes (max %d)", fileInfo.Size(), maxAliasFileSize)
		}
		l.metrics.IncLoadCount(false)
		return nil, command.ErrAliasFileTooLarge
	}

	// Read file content using fs.FS interface
	data, err := fs.ReadFile(l.fsys, l.configPath)
	if err != nil {
		// Wrap permission errors with file path context for better debugging
		if errors.Is(err, fs.ErrPermission) {
			if l.stdLogger != nil {
				l.stdLogger.Printf("[aliasconfig] permission denied reading: %s", l.configPath)
			}
			l.metrics.IncLoadCount(false)
			return nil, fmt.Errorf("aliasconfig: permission denied reading %s: %w", l.configPath, err)
		}
		l.metrics.IncLoadCount(false)
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}

	// Parse YAML
	var config AliasConfig
	if err := yamlUnmarshal(data, &config); err != nil {
		if l.stdLogger != nil {
			l.stdLogger.Printf("[aliasconfig] yaml parsing error: %v", err)
		}
		l.metrics.IncLoadCount(false)
		return nil, fmt.Errorf("%w: %w", command.ErrMalformedAliasFile, err)
	}

	l.logger.Info("alias config loaded", zap.Int("aliases", len(config.Aliases)))
	if l.stdLogger != nil {
		l.stdLogger.Printf("[aliasconfig] loaded %d alias entries", len(config.Aliases))
	}

	d := time.Since(start)
	l.metrics.IncLoadCount(true)
	l.metrics.RecordLoadDuration(d)
	l.metrics.ObserveEntryCount(len(config.Aliases))
	return config.Aliases, nil
}

// LoadDefault loads from the default path with multi-source merge support.
// When both user file and project file exist and MergePolicy is
// MergePolicyProjectOverride (zero value), the two files are merged with
// project entries overriding user entries on conflict, per REQ-AMEND-010.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-010, REQ-AMEND-020
func (l *Loader) LoadDefault() (map[string]string, error) {
	return loadDefaultWithMerge(l)
}

// Validate validates each entry in aliasMap and returns per-entry errors.
// When strict is true, provider and model existence are checked against registry.
// The input map is not mutated. This function signature is preserved from v0.1.0.
//
// For an immutable-input pruning variant, see ValidateAndPrune in policy.go.
func Validate(aliasMap map[string]string, registry *router.ProviderRegistry, strict bool) []error {
	if aliasMap == nil {
		return nil
	}

	var errs []error

	for alias, canonical := range aliasMap {
		// Check for empty alias key
		if alias == "" {
			errs = append(errs, command.ErrEmptyAliasEntry)
			continue
		}

		// Check for empty canonical value
		if canonical == "" {
			errs = append(errs, command.ErrEmptyAliasEntry)
			continue
		}

		// Validate canonical format (provider/model)
		provider, model, ok := parseModelTarget(canonical)
		if !ok {
			errs = append(errs, command.ErrInvalidCanonical)
			continue
		}

		// Strict mode: validate provider and model
		if strict && registry != nil {
			// Check if provider exists in registry
			if _, exists := registry.Get(provider); !exists {
				errs = append(errs, fmt.Errorf("%w: %s", command.ErrUnknownProviderInAlias, provider))
				continue
			}

			// Check if model exists in provider's suggested models
			meta, _ := registry.Get(provider)
			if !slices.Contains(meta.SuggestedModels, model) {
				errs = append(errs, fmt.Errorf("%w: %s", command.ErrUnknownModelInAlias, model))
			}
		}
	}

	return errs
}

// parseModelTarget parses a "provider/model" canonical string.
// Returns (provider, model, true) on success; ("", "", false) on invalid input.
func parseModelTarget(target string) (provider, model string, ok bool) {
	// Exactly one slash required (provider/model)
	slashCount := 0
	slashIndex := -1
	for i := 0; i < len(target); i++ {
		if target[i] == '/' {
			slashCount++
			if slashCount == 1 {
				slashIndex = i
			}
		}
	}

	if slashCount != 1 {
		return "", "", false
	}

	provider = target[:slashIndex]
	model = target[slashIndex+1:]

	if provider == "" || model == "" {
		return "", "", false
	}

	return provider, model, true
}

// yamlUnmarshal unmarshals YAML bytes into an AliasConfig (flat form).
func yamlUnmarshal(data []byte, config *AliasConfig) error {
	return yaml.Unmarshal(data, config)
}

// osFS is an fs.FS implementation backed by the os package.
// Supports absolute paths, unlike os.DirFS which requires a root directory.
type osFS struct{}

// Open implements fs.FS using os.Open, supporting absolute paths.
func (o *osFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}
