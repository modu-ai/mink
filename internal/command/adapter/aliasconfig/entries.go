// Package aliasconfig — AliasEntry extended schema and LoadEntries with yaml union unmarshal.
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-031, REQ-AMEND-040
package aliasconfig

import (
	"errors"
	"fmt"
	"io/fs"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/modu-ai/goose/internal/command"
)

// AliasEntry is the extended form of an alias value.
// Backward-compat: legacy flat string YAML ("alias: provider/model") is
// transparently lifted to AliasEntry{Canonical: "provider/model"} by LoadEntries.
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-031, REQ-AMEND-040
type AliasEntry struct {
	// Canonical is the "provider/model" target, required.
	Canonical string `yaml:"canonical"`

	// Deprecated marks the alias as discouraged. Callers may warn the user.
	Deprecated bool `yaml:"deprecated,omitempty"`

	// ReplacedBy suggests the canonical replacement when Deprecated is true.
	ReplacedBy string `yaml:"replacedBy,omitempty"`

	// ContextWindow overrides the default context window size (in tokens).
	ContextWindow int `yaml:"contextWindow,omitempty"`

	// ProviderHints carries provider-specific hints (key=value pairs).
	ProviderHints []string `yaml:"providerHints,omitempty"`
}

// aliasEntryOrString is an intermediate type used to decode a YAML value
// that can be either a plain string (legacy) or a map (extended AliasEntry).
type aliasEntryOrString struct {
	entry AliasEntry
}

// UnmarshalYAML implements yaml.Unmarshaler for the union type.
// It first attempts to decode as AliasEntry (extended map form).
// On type error, it falls back to decoding as a plain string (legacy flat form)
// and lifts it into AliasEntry{Canonical: s}.
func (u *aliasEntryOrString) UnmarshalYAML(node *yaml.Node) error {
	// Attempt extended map form first.
	var entry AliasEntry
	if err := node.Decode(&entry); err == nil && entry.Canonical != "" {
		u.entry = entry
		return nil
	}

	// Fall back to plain string (legacy flat form).
	var s string
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("aliasconfig: alias value must be a string or extended map: %w", err)
	}
	u.entry = AliasEntry{Canonical: s}
	return nil
}

// aliasEntriesConfig is the YAML document structure for LoadEntries.
// The union unmarshal handles both flat and extended alias values.
type aliasEntriesConfig struct {
	Aliases map[string]aliasEntryOrString `yaml:"aliases"`
}

// LoadEntries reads the alias config file and returns the extended-schema map.
// Legacy flat string entries (e.g. "opus: anthropic/claude-opus-4-7") are
// transparently lifted to AliasEntry{Canonical: ...} with all other fields zero-valued.
//
// Load() continues to return only the canonical string for each entry (backward compat).
//
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 REQ-AMEND-031, REQ-AMEND-040
func (l *Loader) LoadEntries() (map[string]AliasEntry, error) {
	l.logger.Debug("loading alias entries (extended schema)", zap.String("path", l.configPath))

	// Size check.
	fileInfo, err := fs.Stat(l.fsys, l.configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}
	if fileInfo.Size() > maxAliasFileSize {
		return nil, command.ErrAliasFileTooLarge
	}

	data, err := fs.ReadFile(l.fsys, l.configPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}

	var config aliasEntriesConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("%w: %w", command.ErrMalformedAliasFile, err)
	}

	if config.Aliases == nil {
		return nil, nil
	}

	result := make(map[string]AliasEntry, len(config.Aliases))
	for k, v := range config.Aliases {
		result[k] = v.entry
	}
	return result, nil
}
