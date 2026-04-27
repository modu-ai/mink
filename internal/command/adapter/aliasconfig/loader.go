package aliasconfig

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const (
	// maxFileSize is the maximum allowed size for an alias configuration file.
	// REQ-ALIAS-036.
	maxFileSize = 1 << 20 // 1 MiB = 1,048,576 bytes
)

// Options configures Loader behavior.
// All fields are optional; zero values yield usable defaults.
type Options struct {
	// FS is the filesystem interface for reading alias files.
	// If nil, defaults to os.DirFS("/").
	FS fs.FS

	// Logger receives warning logs during loading.
	// If nil, loading is silent.
	Logger *zap.Logger

	// GooseHome overrides the GOOSE_HOME environment variable for testing.
	// If empty, the actual environment is consulted.
	GooseHome string

	// EnvOverrides maps environment variable names to values for testing.
	// If nil, actual os.Getenv is used.
	EnvOverrides map[string]string

	// WorkDir overrides the current working directory for project overlay detection.
	// If empty, os.Getwd() is used.
	WorkDir string
}

// Loader loads alias configuration from YAML files.
// REQ-ALIAS-001.
type Loader struct {
	opts Options
}

// New creates a Loader with the given options.
// Zero options yield a usable loader with OS filesystem and silent logging.
func New(opts Options) *Loader {
	l := &Loader{
		opts: opts,
	}

	// Default FS to OS filesystem if not provided.
	if l.opts.FS == nil {
		l.opts.FS = os.DirFS("/")
	}

	return l
}

// fileSchema is the expected YAML structure for alias files.
type fileSchema struct {
	Aliases map[string]string `yaml:"aliases"`
}

// validateAliases checks that all alias entries are valid.
// Returns ErrEmptyAliasEntry for empty keys or values, and ErrInvalidCanonical
// for canonical values without "/" separator.
func validateAliases(aliases map[string]string) error {
	for key, canonical := range aliases {
		if key == "" {
			return fmt.Errorf("%w: empty alias key", ErrEmptyAliasEntry)
		}
		if canonical == "" {
			return fmt.Errorf("%w: empty canonical value for alias %q", ErrEmptyAliasEntry, key)
		}
		// Check that canonical contains "/" to separate provider and model.
		if !strings.Contains(canonical, "/") {
			return fmt.Errorf("%w: alias %q has canonical %q without provider/model separator", ErrInvalidCanonical, key, canonical)
		}
	}
	return nil
}

// Load reads an alias configuration file from the given path.
// Missing files return an empty map and nil error (REQ-ALIAS-002).
// Empty files return an empty map and nil error (REQ-ALIAS-003).
// REQ-ALIAS-010.
func (l *Loader) Load(path string) (map[string]string, error) {
	// Open the file from the configured filesystem.
	info, err := l.opts.FS.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Missing file is not an error.
			return make(map[string]string), nil
		}
		return nil, err
	}
	defer func() { _ = info.Close() }()

	// Check file size via Stat. fs.File requires Stat(), so this always succeeds.
	fi, err := info.Stat()
	if err != nil {
		return nil, err
	}

	// Prevent DoS via large files. REQ-ALIAS-036.
	if fi.Size() > maxFileSize {
		return nil, ErrAliasFileTooLarge
	}

	var schema fileSchema
	if err := yaml.NewDecoder(info).Decode(&schema); err != nil {
		// Empty file (EOF) is not an error - return empty map.
		if errors.Is(err, io.EOF) {
			return make(map[string]string), nil
		}
		return nil, fmt.Errorf("%w: %v", ErrMalformedAliasFile, err)
	}

	// If aliases field is nil, return empty map.
	if schema.Aliases == nil {
		return make(map[string]string), nil
	}

	// Validate entries
	if err := validateAliases(schema.Aliases); err != nil {
		return nil, err
	}

	return schema.Aliases, nil
}

// getEnv retrieves an environment variable, using overrides if provided for testing.
func (l *Loader) getEnv(key string) string {
	if l.opts.EnvOverrides != nil {
		if val, ok := l.opts.EnvOverrides[key]; ok {
			return val
		}
	}
	return os.Getenv(key)
}

// getwd returns the current working directory, using WorkDir override if provided.
func (l *Loader) getwd() (string, error) {
	if l.opts.WorkDir != "" {
		return l.opts.WorkDir, nil
	}
	return os.Getwd()
}

// LoadDefault loads alias configuration from the standard location search path.
// It searches in the following order:
//  1. $GOOSE_ALIAS_FILE if set (REQ-ALIAS-020)
//  2. $GOOSE_HOME/aliases.yaml if GOOSE_HOME is set (REQ-ALIAS-021)
//  3. $HOME/.goose/aliases.yaml (REQ-ALIAS-022)
//  4. Project overlay at $CWD/.goose/aliases.yaml (REQ-ALIAS-040)
//
// If a project overlay file exists, it is merged with the user config,
// with project entries taking precedence on conflict.
// Missing files at any stage are not errors.
func (l *Loader) LoadDefault() (map[string]string, error) {
	// Start with empty map
	result := make(map[string]string)

	// Step 1: Check GOOSE_ALIAS_FILE
	if aliasFile := l.getEnv("GOOSE_ALIAS_FILE"); aliasFile != "" {
		m, err := l.Load(aliasFile)
		if err != nil {
			return nil, err
		}
		return m, nil // GOOSE_ALIAS_FILE is exclusive, no overlay
	}

	// Step 2: Check GOOSE_HOME/aliases.yaml
	gooseHome := l.opts.GooseHome
	if gooseHome == "" {
		gooseHome = l.getEnv("GOOSE_HOME")
	}
	if gooseHome != "" {
		path := gooseHome + "/aliases.yaml"
		m, err := l.Load(path)
		if err != nil {
			return nil, err
		}
		maps.Copy(result, m)
	}

	// Step 3: Check HOME/.goose/aliases.yaml
	home := l.getEnv("HOME")
	if home != "" && gooseHome == "" {
		// Only check HOME if GOOSE_HOME was not set
		path := home + "/.goose/aliases.yaml"
		m, err := l.Load(path)
		if err != nil {
			return nil, err
		}
		maps.Copy(result, m)
	}

	// Step 4: Check project overlay at CWD/.goose/aliases.yaml
	cwd, err := l.getwd()
	if err == nil {
		projectPath := cwd + "/.goose/aliases.yaml"
		m, err := l.Load(projectPath)
		if err != nil {
			return nil, err
		}
		maps.Copy(result, m)
	}

	return result, nil
}
