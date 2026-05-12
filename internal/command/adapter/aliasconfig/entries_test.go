// Package aliasconfig — entries.go tests (T-006).
// SPEC-GOOSE-ALIAS-CONFIG-001-AMEND-001 AC-AMEND-031, AC-AMEND-040
package aliasconfig

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/modu-ai/mink/internal/command"
)

// writeAliasFile writes the given yaml content under a temp dir and returns
// the absolute path. Caller uses Options.ConfigPath to point Loader at it.
func writeAliasFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "aliases.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write alias file: %v", err)
	}
	return p
}

// TestLoadEntries_LegacyFlatLiftsToCanonical validates the backward-compat
// behavior: a legacy flat-string alias is transparently lifted into
// AliasEntry{Canonical: ...} with all extension fields zero-valued.
// AC-AMEND-031 — REQ-AMEND-031.
func TestLoadEntries_LegacyFlatLiftsToCanonical(t *testing.T) {
	yaml := "aliases:\n  opus: anthropic/claude-opus-4-7\n"
	path := writeAliasFile(t, yaml)
	loader := New(Options{ConfigPath: path})

	got, err := loader.LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	want := AliasEntry{
		Canonical:     "anthropic/claude-opus-4-7",
		Deprecated:    false,
		ReplacedBy:    "",
		ContextWindow: 0,
		ProviderHints: nil,
	}
	if !reflect.DeepEqual(got["opus"], want) {
		t.Fatalf("got %+v, want %+v", got["opus"], want)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
}

// TestLoadEntries_ExtendedSchemaPreservesAllFields validates that the extended
// AliasEntry form (deprecated, replacedBy, contextWindow, providerHints)
// is decoded with no metadata loss.
// AC-AMEND-040 — REQ-AMEND-040.
func TestLoadEntries_ExtendedSchemaPreservesAllFields(t *testing.T) {
	yaml := `aliases:
  opus:
    canonical: anthropic/claude-opus-4-9
    deprecated: true
    replacedBy: anthropic/claude-opus-5-0
    contextWindow: 200000
    providerHints:
      - region=us-east-1
      - tier=premium
`
	path := writeAliasFile(t, yaml)
	loader := New(Options{ConfigPath: path})

	got, err := loader.LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	entry := got["opus"]
	if entry.Canonical != "anthropic/claude-opus-4-9" {
		t.Errorf("canonical = %q", entry.Canonical)
	}
	if !entry.Deprecated {
		t.Errorf("deprecated = false, want true")
	}
	if entry.ReplacedBy != "anthropic/claude-opus-5-0" {
		t.Errorf("replacedBy = %q", entry.ReplacedBy)
	}
	if entry.ContextWindow != 200000 {
		t.Errorf("contextWindow = %d", entry.ContextWindow)
	}
	if len(entry.ProviderHints) != 2 {
		t.Fatalf("providerHints len = %d, want 2", len(entry.ProviderHints))
	}
	if entry.ProviderHints[0] != "region=us-east-1" || entry.ProviderHints[1] != "tier=premium" {
		t.Errorf("providerHints = %v", entry.ProviderHints)
	}
}

// TestLoadEntries_LoadOnLegacyYAMLReturnsCanonicalString validates that the
// legacy Load() method continues to return map[string]string for legacy flat
// yaml — backward-compat for existing callers (no surface change).
// Extended-schema yaml is the LoadEntries contract; Load() retains parent v0.1.0
// behavior of accepting legacy flat strings only (parent FROZEN surface).
func TestLoadEntries_LoadOnLegacyYAMLReturnsCanonicalString(t *testing.T) {
	yaml := "aliases:\n  opus: anthropic/claude-opus-4-9\n"
	path := writeAliasFile(t, yaml)
	loader := New(Options{ConfigPath: path})

	got, err := loader.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got["opus"] != "anthropic/claude-opus-4-9" {
		t.Fatalf("Load[opus] = %q, want anthropic/claude-opus-4-9", got["opus"])
	}
}

// TestLoadEntries_MixedLegacyAndExtendedInSameFile validates the union
// unmarshal handles a YAML document mixing both forms transparently.
// AC-AMEND-031 + AC-AMEND-040 (interaction).
func TestLoadEntries_MixedLegacyAndExtendedInSameFile(t *testing.T) {
	yaml := `aliases:
  opus: anthropic/claude-opus-4-7
  sonnet:
    canonical: anthropic/claude-sonnet-4-6
    contextWindow: 100000
`
	path := writeAliasFile(t, yaml)
	loader := New(Options{ConfigPath: path})

	got, err := loader.LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries: %v", err)
	}
	if got["opus"].Canonical != "anthropic/claude-opus-4-7" || got["opus"].ContextWindow != 0 {
		t.Errorf("opus legacy lift wrong: %+v", got["opus"])
	}
	if got["sonnet"].Canonical != "anthropic/claude-sonnet-4-6" || got["sonnet"].ContextWindow != 100000 {
		t.Errorf("sonnet extended wrong: %+v", got["sonnet"])
	}
}

// TestLoadEntries_FileNotFoundReturnsNil validates that a missing file
// produces (nil, nil) rather than an error — consistent with Load().
// AC-AMEND-031 (graceful path).
func TestLoadEntries_FileNotFoundReturnsNil(t *testing.T) {
	loader := New(Options{ConfigPath: filepath.Join(t.TempDir(), "does-not-exist.yaml")})
	got, err := loader.LoadEntries()
	if err != nil {
		t.Fatalf("expected nil err for missing file, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil map, got %v", got)
	}
}

// TestLoadEntries_MalformedYAMLReturnsParseError validates that a malformed
// document is wrapped with ErrMalformedAliasFile sentinel.
// REQ-AMEND-030 (parse error wrapping for entries path).
func TestLoadEntries_MalformedYAMLReturnsParseError(t *testing.T) {
	yaml := "aliases:\n  opus: [this is not valid yaml\n"
	path := writeAliasFile(t, yaml)
	loader := New(Options{ConfigPath: path})

	_, err := loader.LoadEntries()
	if err == nil {
		t.Fatalf("expected error for malformed yaml, got nil")
	}
	if !errors.Is(err, command.ErrMalformedAliasFile) {
		t.Fatalf("expected ErrMalformedAliasFile, got %v", err)
	}
}

// TestLoadEntries_EmptyFileReturnsNilMap validates that a zero-byte file
// produces (nil, nil) — no entries to surface.
func TestLoadEntries_EmptyFileReturnsNilMap(t *testing.T) {
	path := writeAliasFile(t, "")
	loader := New(Options{ConfigPath: path})
	got, err := loader.LoadEntries()
	if err != nil {
		t.Fatalf("LoadEntries empty: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil map for empty file, got %v", got)
	}
}
