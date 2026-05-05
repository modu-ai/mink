// Package i18n provides locale-aware string catalogs for the TUI.
package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

// writeYAML writes a minimal language.yaml to dir/.moai/config/sections/ and returns the path.
func writeYAML(t *testing.T, dir, lang string) string {
	t.Helper()
	configDir := filepath.Join(dir, ".moai", "config", "sections")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	yamlPath := filepath.Join(configDir, "language.yaml")
	content := "language:\n  conversation_language: " + lang + "\n"
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return yamlPath
}

// TestI18N_Loader_LoadsKoFromYaml verifies that LoadFrom() reads ko catalog when
// language.yaml specifies conversation_language: ko. REQ-CLITUI3-001
func TestI18N_Loader_LoadsKoFromYaml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	yamlPath := writeYAML(t, dir, "ko")

	cat := LoadFrom(yamlPath)
	if cat.Lang != "ko" {
		t.Errorf("LoadFrom().Lang = %q, want %q", cat.Lang, "ko")
	}
}

// TestI18N_Loader_DefaultsToEnglish verifies that LoadFrom() returns en catalog
// when the YAML file does not exist. REQ-CLITUI3-001
func TestI18N_Loader_DefaultsToEnglish(t *testing.T) {
	t.Parallel()

	absent := filepath.Join(t.TempDir(), "nonexistent.yaml")

	cat := LoadFrom(absent)
	if cat.Lang != "en" {
		t.Errorf("LoadFrom(absent).Lang = %q, want %q (default)", cat.Lang, "en")
	}
	if cat != Catalogs["en"] {
		t.Error("LoadFrom(absent) must return en catalog")
	}
}

// TestI18N_Loader_UnknownLangFallback verifies that LoadFrom() returns en catalog
// when the yaml specifies an unknown language code. REQ-CLITUI3-001
func TestI18N_Loader_UnknownLangFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	yamlPath := writeYAML(t, dir, "fr") // "fr" is not in Catalogs

	cat := LoadFrom(yamlPath)
	if cat.Lang != "en" {
		t.Errorf("LoadFrom(fr).Lang = %q, want %q (fallback)", cat.Lang, "en")
	}
}
