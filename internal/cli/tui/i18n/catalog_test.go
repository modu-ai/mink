// Package i18n provides locale-aware string catalogs for the TUI.
package i18n

import (
	"testing"
)

// TestI18N_Catalog_ContainsAllFields verifies that both en and ko catalogs
// have all 13 fields populated with non-empty strings. REQ-CLITUI3-001
func TestI18N_Catalog_ContainsAllFields(t *testing.T) {
	t.Parallel()

	catalogs := map[string]Catalog{
		"en": Catalogs["en"],
		"ko": Catalogs["ko"],
	}

	for lang, cat := range catalogs {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()

			if cat.Lang == "" {
				t.Errorf("Lang must not be empty for %q catalog", lang)
			}
			if cat.StatusbarIdle == "" {
				t.Errorf("StatusbarIdle must not be empty for %q catalog", lang)
			}
			if cat.SessionMenuHeader == "" {
				t.Errorf("SessionMenuHeader must not be empty for %q catalog", lang)
			}
			if cat.SessionMenuEmpty == "" {
				t.Errorf("SessionMenuEmpty must not be empty for %q catalog", lang)
			}
			if cat.EditPrompt == "" {
				t.Errorf("EditPrompt must not be empty for %q catalog", lang)
			}
			if cat.SlashHelpHeader == "" {
				t.Errorf("SlashHelpHeader must not be empty for %q catalog", lang)
			}
			if cat.PermissionPrompt == "" {
				t.Errorf("PermissionPrompt must not be empty for %q catalog", lang)
			}
			if cat.PermissionAllowOnce == "" {
				t.Errorf("PermissionAllowOnce must not be empty for %q catalog", lang)
			}
			if cat.PermissionAllowAlways == "" {
				t.Errorf("PermissionAllowAlways must not be empty for %q catalog", lang)
			}
			if cat.PermissionDenyOnce == "" {
				t.Errorf("PermissionDenyOnce must not be empty for %q catalog", lang)
			}
			if cat.PermissionDenyAlways == "" {
				t.Errorf("PermissionDenyAlways must not be empty for %q catalog", lang)
			}
			if cat.Saved == "" {
				t.Errorf("Saved must not be empty for %q catalog", lang)
			}
			if cat.Loaded == "" {
				t.Errorf("Loaded must not be empty for %q catalog", lang)
			}
		})
	}
}

// TestI18N_Catalog_EnSavedMatchesFormat verifies the en Saved format string
// produces byte-identical output to the previously hardcoded string. REQ-CLITUI3-001
func TestI18N_Catalog_EnSavedMatchesFormat(t *testing.T) {
	t.Parallel()

	cat := Catalogs["en"]
	if cat.Saved != "[saved: %s]" {
		t.Errorf("en Saved = %q, want %q", cat.Saved, "[saved: %s]")
	}
}

// TestI18N_Default_ReturnsEnCatalog verifies that Default() returns the en catalog.
func TestI18N_Default_ReturnsEnCatalog(t *testing.T) {
	t.Parallel()

	def := Default()
	if def.Lang != "en" {
		t.Errorf("Default().Lang = %q, want %q", def.Lang, "en")
	}
	if def != Catalogs["en"] {
		t.Error("Default() must return en catalog")
	}
}
