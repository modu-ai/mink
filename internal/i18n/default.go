package i18n

import (
	"context"
	"embed"
	"log"
	"sync"

	"github.com/modu-ai/mink/internal/locale"
)

// catalogFS embeds all YAML files from the catalog directory at compile time.
// This satisfies REQ-I18N-016: Tier 1/Tier 2 bundles must not require network I/O.
//
//go:embed all:catalog/*.yaml
var catalogFS embed.FS

var (
	defaultOnce   sync.Once
	defaultBundle *Bundle
	defaultErr    error
)

// initDefaultBundle creates the process-wide Bundle from the embedded catalog.
// Called lazily by Default(). Panics only if the embedded catalog is corrupted
// (which would indicate a broken build, not a runtime condition).
func initDefaultBundle() {
	b := NewBundle("en")
	if err := b.LoadFS(catalogFS, "catalog"); err != nil {
		defaultErr = err
		log.Printf("[i18n] failed to load embedded catalog: %v", err)
		// Degrade gracefully: the bundle is still usable for en fallback via
		// the key-as-fallback path in Translator.
	}
	defaultBundle = b
}

// Default returns a Translator for the language detected from the current
// process environment via locale.Detect. Detection is best-effort; on any
// error it falls back to English.
//
// The Bundle is initialised exactly once per process lifetime (sync.Once).
// Default is safe for concurrent use.
func Default() Translator {
	return DefaultFor(context.Background())
}

// DefaultFor returns a Translator for the language detected from ctx.
// It is preferred over Default() when a context.Context is already available
// (e.g., in HTTP handlers or CLI command RunE functions).
func DefaultFor(ctx context.Context) Translator {
	defaultOnce.Do(initDefaultBundle)

	lang := "en"
	if lc, err := locale.Detect(ctx); err == nil && lc.PrimaryLanguage != "" {
		lang = lc.PrimaryLanguage
	}
	return defaultBundle.Translator(lang)
}
