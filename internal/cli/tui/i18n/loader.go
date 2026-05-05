// Package i18n provides locale-aware string catalogs for the TUI.
package i18n

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// languageYAMLPath is the relative path from a project root to the language config.
const languageYAMLPath = ".moai/config/sections/language.yaml"

// languageConfig is the YAML structure for language.yaml.
type languageConfig struct {
	Language struct {
		ConversationLanguage string `yaml:"conversation_language"`
	} `yaml:"language"`
}

// Load returns the Catalog for the conversation_language in the project config.
//
// Lookup order:
//  1. CWD/.moai/config/sections/language.yaml
//  2. git-toplevel/.moai/config/sections/language.yaml
//  3. "en" default (silent fallback)
//
// Returns the English catalog when:
//   - file not found at either path
//   - key missing or empty
//   - language code not in Catalogs map
//
// @MX:ANCHOR Load is the primary catalog entry point used by model.go Init().
// @MX:REASON fan_in >= 3: model.go Init(), loader_test.go (3 test cases), future snapshots test
func Load() Catalog {
	lang := loadLang()
	if lang == "" {
		return Default()
	}
	if cat, ok := Catalogs[lang]; ok {
		return cat
	}
	return Default()
}

// loadLang resolves the conversation_language string from the YAML config.
// Returns empty string on any error or missing key.
func loadLang() string {
	// Tier 1: CWD-relative path.
	cwd, err := os.Getwd()
	if err == nil {
		if lang := readLangFromFile(filepath.Join(cwd, languageYAMLPath)); lang != "" {
			return lang
		}
	}

	// Tier 2: git toplevel.
	if root := gitToplevel(); root != "" {
		if lang := readLangFromFile(filepath.Join(root, languageYAMLPath)); lang != "" {
			return lang
		}
	}

	return ""
}

// readLangFromFile reads conversation_language from a language.yaml file.
// Returns empty string when the file does not exist, cannot be parsed,
// or the key is absent.
func readLangFromFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg languageConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.Language.ConversationLanguage)
}

// gitToplevel returns the git repository root directory by running
// `git rev-parse --show-toplevel`. Returns empty string on failure.
func gitToplevel() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
