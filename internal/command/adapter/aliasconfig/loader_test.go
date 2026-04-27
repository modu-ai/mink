package aliasconfig

import (
	"errors"
	"testing"
	"testing/fstest"
)

// TestLoader_Load verifies basic Load behavior.
// REQ-ALIAS-002 (missing file), REQ-ALIAS-003 (empty file), REQ-ALIAS-010 (valid file).
func TestLoader_Load(t *testing.T) {
	t.Run("missing file returns empty map and nil error", func(t *testing.T) {
		// REQ-ALIAS-002
		fs := fstest.MapFS{}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("nonexistent/aliases.yaml")

		if err != nil {
			t.Errorf("Load() on missing file should return nil error, got %v", err)
		}
		if aliasMap == nil {
			t.Error("Load() on missing file should return empty map (non-nil), got nil")
		}
		if len(aliasMap) != 0 {
			t.Errorf("Load() on missing file should return empty map, got %d entries", len(aliasMap))
		}
	})

	t.Run("empty file returns empty map and nil error", func(t *testing.T) {
		// REQ-ALIAS-003
		fs := fstest.MapFS{
			"empty.yaml": &fstest.MapFile{Data: []byte("")},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("empty.yaml")

		if err != nil {
			t.Errorf("Load() on empty file should return nil error, got %v", err)
		}
		if aliasMap == nil {
			t.Error("Load() on empty file should return empty map (non-nil), got nil")
		}
		if len(aliasMap) != 0 {
			t.Errorf("Load() on empty file should return empty map, got %d entries", len(aliasMap))
		}
	})

	t.Run("empty aliases object returns empty map and nil error", func(t *testing.T) {
		// REQ-ALIAS-003
		fs := fstest.MapFS{
			"empty_aliases.yaml": &fstest.MapFile{Data: []byte("aliases: {}\n")},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("empty_aliases.yaml")

		if err != nil {
			t.Errorf("Load() on empty aliases object should return nil error, got %v", err)
		}
		if aliasMap == nil {
			t.Error("Load() on empty aliases object should return empty map (non-nil), got nil")
		}
		if len(aliasMap) != 0 {
			t.Errorf("Load() on empty aliases object should return empty map, got %d entries", len(aliasMap))
		}
	})

	t.Run("valid file returns parsed map", func(t *testing.T) {
		// REQ-ALIAS-010
		fs := fstest.MapFS{
			"valid.yaml": &fstest.MapFile{Data: []byte(`
aliases:
  opus: anthropic/claude-opus-4-7
  sonnet: anthropic/claude-sonnet-4-6
`)},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("valid.yaml")

		if err != nil {
			t.Errorf("Load() on valid file should return nil error, got %v", err)
		}
		if aliasMap == nil {
			t.Fatal("Load() on valid file should return non-nil map")
		}
		if len(aliasMap) != 2 {
			t.Errorf("Load() on valid file should return 2 entries, got %d", len(aliasMap))
		}
		if got, want := aliasMap["opus"], "anthropic/claude-opus-4-7"; got != want {
			t.Errorf("aliasMap[\"opus\"] = %q, want %q", got, want)
		}
		if got, want := aliasMap["sonnet"], "anthropic/claude-sonnet-4-6"; got != want {
			t.Errorf("aliasMap[\"sonnet\"] = %q, want %q", got, want)
		}
	})

	t.Run("malformed YAML returns ErrMalformedAliasFile", func(t *testing.T) {
		// REQ-ALIAS-030
		fs := fstest.MapFS{
			"malformed.yaml": &fstest.MapFile{Data: []byte(`
aliases:
  - item1
  - item2
`)},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("malformed.yaml")

		if err == nil {
			t.Error("Load() on malformed YAML should return error, got nil")
		}
		if !errors.Is(err, ErrMalformedAliasFile) {
			t.Errorf("Load() on malformed YAML should return ErrMalformedAliasFile, got %v", err)
		}
		if aliasMap != nil {
			t.Error("Load() on malformed YAML should return nil map, got non-nil")
		}
	})

	t.Run("empty alias key returns ErrEmptyAliasEntry", func(t *testing.T) {
		// REQ-ALIAS-031
		fs := fstest.MapFS{
			"empty_key.yaml": &fstest.MapFile{Data: []byte(`
aliases:
  "": anthropic/claude-opus-4-7
`)},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("empty_key.yaml")

		if err == nil {
			t.Error("Load() with empty alias key should return error, got nil")
		}
		if !errors.Is(err, ErrEmptyAliasEntry) {
			t.Errorf("Load() with empty alias key should return ErrEmptyAliasEntry, got %v", err)
		}
		if aliasMap != nil {
			t.Error("Load() with empty alias key should return nil map, got non-nil")
		}
	})

	t.Run("empty canonical value returns ErrEmptyAliasEntry", func(t *testing.T) {
		// REQ-ALIAS-032
		fs := fstest.MapFS{
			"empty_value.yaml": &fstest.MapFile{Data: []byte(`
aliases:
  opus: ""
`)},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("empty_value.yaml")

		if err == nil {
			t.Error("Load() with empty canonical value should return error, got nil")
		}
		if !errors.Is(err, ErrEmptyAliasEntry) {
			t.Errorf("Load() with empty canonical value should return ErrEmptyAliasEntry, got %v", err)
		}
		if aliasMap != nil {
			t.Error("Load() with empty canonical value should return nil map, got non-nil")
		}
	})

	t.Run("canonical without slash returns ErrInvalidCanonical", func(t *testing.T) {
		// REQ-ALIAS-033
		fs := fstest.MapFS{
			"no_slash.yaml": &fstest.MapFile{Data: []byte(`
aliases:
  opus: claude-opus-4-7
`)},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("no_slash.yaml")

		if err == nil {
			t.Error("Load() with canonical without slash should return error, got nil")
		}
		if !errors.Is(err, ErrInvalidCanonical) {
			t.Errorf("Load() with canonical without slash should return ErrInvalidCanonical, got %v", err)
		}
		if aliasMap != nil {
			t.Error("Load() with canonical without slash should return nil map, got non-nil")
		}
	})

	t.Run("file larger than 1 MiB returns ErrAliasFileTooLarge", func(t *testing.T) {
		// REQ-ALIAS-036: 1,048,576 bytes = 1 MiB
		largeData := make([]byte, 1_048_577) // One byte over the limit
		for i := range largeData {
			largeData[i] = 'x'
		}
		fs := fstest.MapFS{
			"large.yaml": &fstest.MapFile{Data: largeData},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("large.yaml")

		if err == nil {
			t.Error("Load() on oversized file should return error, got nil")
		}
		if !errors.Is(err, ErrAliasFileTooLarge) {
			t.Errorf("Load() on oversized file should return ErrAliasFileTooLarge, got %v", err)
		}
		if aliasMap != nil {
			t.Error("Load() on oversized file should return nil map, got non-nil")
		}
	})

	t.Run("file exactly 1 MiB is accepted", func(t *testing.T) {
		// Boundary test: exactly 1 MiB should be OK
		exactData := make([]byte, 1_048_576)
		for i := range exactData {
			exactData[i] = 'x'
		}
		fs := fstest.MapFS{
			"exact.yaml": &fstest.MapFile{Data: exactData},
		}
		loader := New(Options{FS: fs})

		// File will fail to parse as YAML (just 'x' characters), but size check should pass
		_, err := loader.Load("exact.yaml")

		// Should fail with ErrMalformedAliasFile (YAML parse error), not ErrAliasFileTooLarge
		if err == nil {
			t.Error("Load() on invalid YAML should return error, got nil")
		}
		if errors.Is(err, ErrAliasFileTooLarge) {
			t.Error("Load() on file exactly 1 MiB should not return ErrAliasFileTooLarge")
		}
	})

	t.Run("YAML without aliases key returns empty map", func(t *testing.T) {
		// File with valid YAML but no aliases field
		fs := fstest.MapFS{
			"no_aliases.yaml": &fstest.MapFile{Data: []byte("something_else: true\n")},
		}
		loader := New(Options{FS: fs})

		aliasMap, err := loader.Load("no_aliases.yaml")
		if err != nil {
			t.Fatalf("Load() on file without aliases key should succeed, got %v", err)
		}
		if len(aliasMap) != 0 {
			t.Errorf("Load() on file without aliases key should return empty map, got %d entries", len(aliasMap))
		}
	})
}

// TestLoader_LoadDefault verifies path resolution and environment variable handling.
// REQ-ALIAS-020 (GOOSE_ALIAS_FILE), REQ-ALIAS-021 (GOOSE_HOME), REQ-ALIAS-022 (HOME fallback).
func TestLoader_LoadDefault(t *testing.T) {
	t.Run("GOOSE_ALIAS_FILE takes precedence", func(t *testing.T) {
		// REQ-ALIAS-020
		fs := fstest.MapFS{
			"custom/aliases.yaml": &fstest.MapFile{Data: []byte("aliases:\n  opus: anthropic/claude-opus-4-7\n")},
		}
		loader := New(Options{
			FS: fs,
			EnvOverrides: map[string]string{
				"GOOSE_ALIAS_FILE": "custom/aliases.yaml",
				"GOOSE_HOME":       "goose",
				"HOME":             "user",
			},
		})

		aliasMap, err := loader.LoadDefault()

		if err != nil {
			t.Errorf("LoadDefault() should succeed, got %v", err)
		}
		if len(aliasMap) != 1 {
			t.Errorf("LoadDefault() should return 1 entry, got %d", len(aliasMap))
		}
	})

	t.Run("GOOSE_HOME when GOOSE_ALIAS_FILE not set", func(t *testing.T) {
		// REQ-ALIAS-021
		fs := fstest.MapFS{
			"goose/aliases.yaml": &fstest.MapFile{Data: []byte("aliases:\n  opus: anthropic/claude-opus-4-7\n")},
		}
		loader := New(Options{
			FS: fs,
			EnvOverrides: map[string]string{
				"GOOSE_HOME": "goose",
				"HOME":       "user",
			},
		})

		aliasMap, err := loader.LoadDefault()

		if err != nil {
			t.Errorf("LoadDefault() should succeed, got %v", err)
		}
		if len(aliasMap) != 1 {
			t.Errorf("LoadDefault() should return 1 entry, got %d", len(aliasMap))
		}
	})

	t.Run("HOME fallback when neither env var set", func(t *testing.T) {
		// REQ-ALIAS-022
		fs := fstest.MapFS{
			"user/.goose/aliases.yaml": &fstest.MapFile{Data: []byte("aliases:\n  opus: anthropic/claude-opus-4-7\n")},
		}
		loader := New(Options{
			FS: fs,
			EnvOverrides: map[string]string{
				"HOME": "user",
			},
		})

		aliasMap, err := loader.LoadDefault()

		if err != nil {
			t.Errorf("LoadDefault() should succeed, got %v", err)
		}
		if len(aliasMap) != 1 {
			t.Errorf("LoadDefault() should return 1 entry, got %d", len(aliasMap))
		}
	})

	t.Run("missing file returns empty map and nil error", func(t *testing.T) {
		// All paths missing should not error
		fs := fstest.MapFS{}
		loader := New(Options{
			FS: fs,
			EnvOverrides: map[string]string{
				"HOME": "user",
			},
		})

		aliasMap, err := loader.LoadDefault()

		if err != nil {
			t.Errorf("LoadDefault() with missing files should return nil error, got %v", err)
		}
		if aliasMap == nil {
			t.Error("LoadDefault() with missing files should return empty map (non-nil), got nil")
		}
		if len(aliasMap) != 0 {
			t.Errorf("LoadDefault() with missing files should return empty map, got %d entries", len(aliasMap))
		}
	})

	t.Run("project overlay merges with user config", func(t *testing.T) {
		// REQ-ALIAS-040: project overlay at CWD/.goose/aliases.yaml
		fs := fstest.MapFS{
			"user/.goose/aliases.yaml":    &fstest.MapFile{Data: []byte("aliases:\n  opus: anthropic/claude-opus-4-7\n")},
			"project/.goose/aliases.yaml": &fstest.MapFile{Data: []byte("aliases:\n  sonnet: anthropic/claude-sonnet-4-6\n")},
		}
		loader := New(Options{
			FS:      fs,
			WorkDir: "project",
			EnvOverrides: map[string]string{
				"HOME": "user",
			},
		})

		aliasMap, err := loader.LoadDefault()

		if err != nil {
			t.Errorf("LoadDefault() with project overlay should succeed, got %v", err)
		}
		// Both entries should be present, with project override taking precedence
		if len(aliasMap) != 2 {
			t.Errorf("LoadDefault() with project overlay should return 2 entries, got %d", len(aliasMap))
		}
	})
}

// TestLoader_LoadDefault_Errors verifies error propagation from LoadDefault paths.
func TestLoader_LoadDefault_Errors(t *testing.T) {
	malformedYAML := []byte("aliases:\n  - list\n")

	t.Run("GOOSE_ALIAS_FILE errors propagate", func(t *testing.T) {
		fsys := fstest.MapFS{
			"custom/aliases.yaml": &fstest.MapFile{Data: malformedYAML},
		}
		loader := New(Options{
			FS: fsys,
			EnvOverrides: map[string]string{
				"GOOSE_ALIAS_FILE": "custom/aliases.yaml",
			},
		})

		_, err := loader.LoadDefault()
		if err == nil {
			t.Fatal("LoadDefault() with malformed GOOSE_ALIAS_FILE should return error")
		}
		if !errors.Is(err, ErrMalformedAliasFile) {
			t.Errorf("LoadDefault() should propagate ErrMalformedAliasFile, got %v", err)
		}
	})

	t.Run("GOOSE_HOME errors propagate", func(t *testing.T) {
		fsys := fstest.MapFS{
			"goose/aliases.yaml": &fstest.MapFile{Data: malformedYAML},
		}
		loader := New(Options{
			FS: fsys,
			EnvOverrides: map[string]string{
				"GOOSE_HOME": "goose",
			},
		})

		_, err := loader.LoadDefault()
		if err == nil {
			t.Fatal("LoadDefault() with malformed GOOSE_HOME file should return error")
		}
		if !errors.Is(err, ErrMalformedAliasFile) {
			t.Errorf("LoadDefault() should propagate ErrMalformedAliasFile, got %v", err)
		}
	})

	t.Run("HOME errors propagate", func(t *testing.T) {
		fsys := fstest.MapFS{
			"user/.goose/aliases.yaml": &fstest.MapFile{Data: malformedYAML},
		}
		loader := New(Options{
			FS: fsys,
			EnvOverrides: map[string]string{
				"HOME": "user",
			},
		})

		_, err := loader.LoadDefault()
		if err == nil {
			t.Fatal("LoadDefault() with malformed HOME file should return error")
		}
		if !errors.Is(err, ErrMalformedAliasFile) {
			t.Errorf("LoadDefault() should propagate ErrMalformedAliasFile, got %v", err)
		}
	})

	t.Run("project overlay errors propagate", func(t *testing.T) {
		fsys := fstest.MapFS{
			"project/.goose/aliases.yaml": &fstest.MapFile{Data: malformedYAML},
		}
		loader := New(Options{
			FS:      fsys,
			WorkDir: "project",
			EnvOverrides: map[string]string{
				"HOME": "missing",
			},
		})

		_, err := loader.LoadDefault()
		if err == nil {
			t.Fatal("LoadDefault() with malformed project overlay should return error")
		}
		if !errors.Is(err, ErrMalformedAliasFile) {
			t.Errorf("LoadDefault() should propagate ErrMalformedAliasFile, got %v", err)
		}
	})
}

// TestLoader_New verifies Loader construction.
// REQ-ALIAS-001.
func TestLoader_New(t *testing.T) {
	t.Run("New creates a Loader with options", func(t *testing.T) {
		opts := Options{
			FS:     fstest.MapFS{},
			Logger: nil,
		}
		loader := New(opts)

		if loader == nil {
			t.Fatal("New() returned nil")
		}
		// Verify loader.opts is populated
		if loader.opts.FS == nil {
			t.Error("Loader opts.FS is nil, expected non-nil")
		}
	})

	t.Run("New with zero Options returns usable Loader", func(t *testing.T) {
		loader := New(Options{})

		if loader == nil {
			t.Fatal("New(Options{}) returned nil")
		}
		// Zero options should use defaults: FS = os FS, Logger = nil
		if loader.opts.FS == nil {
			t.Error("Loader with zero opts should default FS to OS filesystem")
		}
	})
}

// TestOptionsDefaults verifies Options field defaults.
func TestOptionsDefaults(t *testing.T) {
	t.Run("FS defaults to OS filesystem when nil", func(t *testing.T) {
		opts := Options{}
		loader := New(opts)

		// loader.opts.FS should be non-nil (os.DirFS)
		if loader.opts.FS == nil {
			t.Error("Expected FS to default to os.DirFS, got nil")
		}
	})

	t.Run("nil Logger is allowed", func(t *testing.T) {
		opts := Options{
			Logger: nil,
		}
		loader := New(opts)

		if loader.opts.Logger != nil {
			t.Error("Expected nil Logger to be preserved, got non-nil")
		}
	})
}
