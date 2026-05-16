package i18n

import (
	"fmt"
	"io/fs"
	"path/filepath"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Bundle wraps go-i18n/v2's *i18n.Bundle and provides YAML-first loading helpers.
// Construct one Bundle per application and share it across goroutines.
//
// @MX:ANCHOR: [AUTO] Central entry point for all translation loading — consumed by Default, tests, CLI init.
// @MX:REASON: Changing NewBundle signature or LoadDirectory semantics breaks the Default() singleton and all test fixtures.
type Bundle struct {
	inner       *goi18n.Bundle
	defaultLang string
}

// NewBundle creates an empty Bundle configured for the given default language tag.
// The defaultLang must be a valid BCP 47 tag (e.g., "en", "en-US", "ko").
// YAML is the only unmarshaler registered; JSON/TOML are not used by MINK.
func NewBundle(defaultLang string) *Bundle {
	b := &Bundle{
		inner:       goi18n.NewBundle(language.Make(defaultLang)),
		defaultLang: defaultLang,
	}
	b.inner.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	return b
}

// LoadMessageFile parses a single YAML translation file.
// The file name must encode the language tag as its base name (without extension),
// e.g., "en.yaml" or "ko.yaml". The loader rejects BOM-prefixed and CRLF files.
// On validation failure the file is skipped without crashing; an error is returned.
func (b *Bundle) LoadMessageFile(path string) error {
	return b.loadPath(path)
}

// LoadDirectory loads all *.yaml files from the given directory.
// Files that fail validation are skipped and their errors are collected.
// If at least one file loads successfully this method returns nil.
// If no file loads, the first encountered error is returned.
func (b *Bundle) LoadDirectory(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return fmt.Errorf("i18n: glob %s: %w", dir, err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("i18n: no *.yaml files found in %s", dir)
	}

	var firstErr error
	loaded := 0
	for _, path := range matches {
		if loadErr := b.loadPath(path); loadErr != nil {
			if firstErr == nil {
				firstErr = loadErr
			}
			continue
		}
		loaded++
	}

	if loaded == 0 {
		return firstErr
	}
	return nil
}

// LoadFS loads all *.yaml files from an fs.FS subtree (e.g., embed.FS).
// This is the path used by Default() via go:embed.
func (b *Bundle) LoadFS(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("i18n: read dir %s: %w", dir, err)
	}

	var firstErr error
	loaded := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, readErr := fs.ReadFile(fsys, dir+"/"+e.Name())
		if readErr != nil {
			if firstErr == nil {
				firstErr = readErr
			}
			continue
		}
		// Determine language tag from file name (strip .yaml suffix).
		lang := e.Name()[:len(e.Name())-5]
		if loadErr := b.loadBytes(data, lang+".yaml"); loadErr != nil {
			if firstErr == nil {
				firstErr = loadErr
			}
			continue
		}
		loaded++
	}

	if loaded == 0 {
		return firstErr
	}
	return nil
}

// Translator returns a per-language Translator backed by go-i18n's *Localizer.
// If no bundle was loaded for lang, it attempts BCP 47 truncation (e.g., "fr-CA" → "fr")
// before falling back to defaultLang. If defaultLang is also missing, the Translator
// still functions but always returns the key string (never panics).
func (b *Bundle) Translator(lang string) Translator {
	return &localizer{
		loc:  goi18n.NewLocalizer(b.inner, lang, b.defaultLang),
		lang: lang,
	}
}

// --------------------------------------------------------------------------
// internal helpers
// --------------------------------------------------------------------------

// loadPath reads the file at path and delegates to loadBytes.
func (b *Bundle) loadPath(path string) error {
	data, err := readFileStrict(path)
	if err != nil {
		return err
	}
	return b.loadBytes(data, filepath.Base(path))
}

// loadBytes registers the raw YAML bytes in go-i18n's bundle.
// The name parameter must end in ".yaml"; the basename (without extension) is
// used as the BCP 47 language tag.
func (b *Bundle) loadBytes(data []byte, name string) error {
	if _, parseErr := b.inner.ParseMessageFileBytes(data, name); parseErr != nil {
		return fmt.Errorf("i18n: parse %s: %w", name, parseErr)
	}
	return nil
}
