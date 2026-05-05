// Package i18n provides locale-aware string catalogs for the TUI.
package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

// TestI18N_Loader_LoadsKoFromYaml verifies that Load() reads ko catalog when
// language.yaml specifies conversation_language: ko. REQ-CLITUI3-001
func TestI18N_Loader_LoadsKoFromYaml(t *testing.T) {
	t.Parallel()

	// Create a temp dir with a language.yaml file.
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".moai", "config", "sections")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	yamlContent := "language:\n  conversation_language: ko\n"
	yamlPath := filepath.Join(configDir, "language.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write language.yaml: %v", err)
	}

	// Set CWD to the temp dir so Load() finds the yaml.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cat := Load()
	if cat.Lang != "ko" {
		t.Errorf("Load().Lang = %q, want %q", cat.Lang, "ko")
	}
}

// TestI18N_Loader_DefaultsToEnglish verifies that Load() returns en catalog
// when no language.yaml is found. REQ-CLITUI3-001
func TestI18N_Loader_DefaultsToEnglish(t *testing.T) {
	t.Parallel()

	// Use a temp dir with no yaml file.
	dir := t.TempDir()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cat := Load()
	if cat.Lang != "en" {
		t.Errorf("Load().Lang = %q, want %q (default fallback)", cat.Lang, "en")
	}
	if cat != Catalogs["en"] {
		t.Error("Load() must return en catalog when yaml is absent")
	}
}

// TestI18N_Loader_UnknownLangFallback verifies that Load() returns en catalog
// when the yaml specifies an unknown language code. REQ-CLITUI3-001
func TestI18N_Loader_UnknownLangFallback(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDir := filepath.Join(dir, ".moai", "config", "sections")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	// "fr" is not in Catalogs.
	yamlContent := "language:\n  conversation_language: fr\n"
	yamlPath := filepath.Join(configDir, "language.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write language.yaml: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cat := Load()
	if cat.Lang != "en" {
		t.Errorf("Load().Lang = %q, want %q (fallback for unknown lang)", cat.Lang, "en")
	}
}
