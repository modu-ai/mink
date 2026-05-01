// Package aliasconfig — multi-source file merge logic for LoadDefault.
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-010, REQ-AMEND-020
package aliasconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// userAliasPath returns the user-scope aliases.yaml path using the same
// fallback chain as New(opts) when opts.ConfigPath is empty.
// Returns empty string when no home directory can be determined.
func userAliasPath() string {
	if home := os.Getenv(homeEnv); home != "" {
		return filepath.Join(home, "aliases.yaml")
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, defaultConfigPath)
}

// projectAliasPath returns $CWD/.goose/aliases.yaml.
// Returns empty string when the working directory cannot be determined.
func projectAliasPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, ".goose", "aliases.yaml")
}

// fileExists reports whether the given path exists in the real filesystem.
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// loadFileFromFS reads and parses an alias file using the provided fs.FS.
// Returns (nil, nil) when the file does not exist.
func loadFileFromFS(fsys fs.FS, path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}

	info, err := fs.Stat(fsys, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}

	if info.Size() > maxAliasFileSize {
		return nil, errAliasFileTooLargeAt(path)
	}

	data, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}

	var config AliasConfig
	if err := yamlUnmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("aliasconfig: malformed YAML in %s: %w", path, err)
	}

	return config.Aliases, nil
}

// errAliasFileTooLargeAt wraps the file-too-large sentinel with path context.
func errAliasFileTooLargeAt(path string) error {
	return fmt.Errorf("aliasconfig: alias file too large at %s: %w", path, errFileTooLarge)
}

// errFileTooLarge is the internal sentinel for size-limit errors.
// This avoids importing command to keep merge.go thin.
// The caller (Load) already uses command.ErrAliasFileTooLarge directly.
var errFileTooLarge = fmt.Errorf("file exceeds 1 MiB cap")

// loadDefaultWithMerge implements the MergePolicy-aware LoadDefault logic.
// It is the single implementation called by Loader.LoadDefault.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-010, REQ-AMEND-020
func loadDefaultWithMerge(l *Loader) (map[string]string, error) {
	userPath := userAliasPath()
	projPath := projectAliasPath()

	switch l.mergePolicy {
	case MergePolicyUserOnly:
		// Load only from the user-scope file, ignoring project file.
		// Build a temporary loader that points at userPath for correct metrics wiring.
		return loadSinglePath(l, userPath)

	case MergePolicyProjectOnly:
		// Load only from the project file, ignoring user file.
		// Fall back to l.Load() when no project file exists.
		if projPath == "" || !fileExists(projPath) {
			return l.Load()
		}
		return loadSinglePath(l, projPath)
	}

	// MergePolicyProjectOverride (default): check whether both sources exist.
	userExists := fileExists(userPath)
	projExists := fileExists(projPath)

	// When only one source exists, delegate to Load for correct metrics wiring.
	if !userExists || !projExists {
		return l.Load()
	}

	// Both files exist — perform the merge.
	return mergeUserAndProject(l, userPath, projPath)
}

// loadSinglePath reads a single alias file at the given path and emits metrics.
// This helper is used by UserOnly / ProjectOnly policies.
func loadSinglePath(l *Loader, path string) (map[string]string, error) {
	if path == "" || !fileExists(path) {
		l.metrics.IncLoadCount(true)
		l.metrics.RecordLoadDuration(0)
		l.metrics.ObserveEntryCount(0)
		return nil, nil
	}

	start := time.Now()
	m, err := loadFileFromFS(l.fsys, path)
	if err != nil {
		l.metrics.IncLoadCount(false)
		return nil, err
	}

	d := time.Since(start)
	l.metrics.IncLoadCount(true)
	l.metrics.RecordLoadDuration(d)
	l.metrics.ObserveEntryCount(len(m))
	return m, nil
}

// mergeUserAndProject loads both files and overlays project entries on top of user entries.
// Logs one info entry per overridden key (REQ-AMEND-010).
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-010-A, REQ-AMEND-010-B
func mergeUserAndProject(l *Loader, userPath, projPath string) (map[string]string, error) {
	start := time.Now()

	userMap, err := loadFileFromFS(l.fsys, userPath)
	if err != nil {
		l.metrics.IncLoadCount(false)
		return nil, err
	}

	projMap, err := loadFileFromFS(l.fsys, projPath)
	if err != nil {
		l.metrics.IncLoadCount(false)
		return nil, err
	}

	// Build merged map: start with user entries, then overlay project entries.
	merged := make(map[string]string, len(userMap)+len(projMap))
	for k, v := range userMap {
		merged[k] = v
	}

	// Overlay project entries; log each key that overrides a user entry.
	for k, v := range projMap {
		if _, overriding := merged[k]; overriding {
			l.logger.Info(
				"alias overridden by project file",
				zap.String("alias", k),
				zap.String("user_file", userPath),
				zap.String("project_file", projPath),
			)
		}
		merged[k] = v
	}

	d := time.Since(start)
	l.metrics.IncLoadCount(true)
	l.metrics.RecordLoadDuration(d)
	l.metrics.ObserveEntryCount(len(merged))

	return merged, nil
}
