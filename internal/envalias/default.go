package envalias

import (
	"os"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

// Default is the package-level loader initialised by Init.
// It is stored via atomic.Pointer to guarantee safe concurrent reads after Init completes.
//
// @MX:ANCHOR: [AUTO] package-level Default loader — all DefaultGet callers read through this
// @MX:REASON: fan_in >= 11 (Phase 3 distributed read sites); atomic.Pointer guards concurrent access
var defaultPtr atomic.Pointer[Loader]

// initOnce ensures Init runs the loader-construction logic at most once per process.
var initOnce sync.Once

// Init constructs a Loader with the given zap.Logger and stores it as the package Default.
// Subsequent calls are no-ops; the first logger wins (sync.Once semantics).
// Returns the initialised Loader so callers can capture it for testing.
//
// @MX:WARN: [AUTO] Init は sync.Once で保護されているが logger nil 渡し可 — 本番コードでは必ず非nilロガーを渡す
// @MX:REASON: nil logger silently suppresses all deprecation/conflict warnings
func Init(logger *zap.Logger) *Loader {
	initOnce.Do(func() {
		l := New(Options{Logger: logger})
		defaultPtr.Store(l)
	})
	return defaultPtr.Load()
}

// DefaultGet is a convenience accessor for the package-level Default loader.
//
// Safe-fallback behaviour when Init has not been called (Default is nil):
//   - Looks up os.Getenv("MINK_" + key) then os.Getenv("GOOSE_" + key).
//   - No deprecation warning is emitted (no logger available).
//   - Returns appropriate EnvSource without panicking.
//
// When Default is initialised, delegates to Default.Get(key).
func DefaultGet(key string) (value string, source EnvSource, ok bool) {
	l := defaultPtr.Load()
	if l == nil {
		return fallbackGet(key)
	}
	return l.Get(key)
}

// fallbackGet performs a direct os.Getenv lookup against the canonical key pair
// without needing an initialised Loader. Used by DefaultGet before Init is called.
func fallbackGet(key string) (string, EnvSource, bool) {
	pair, registered := keyMappings[key]
	if !registered {
		return "", SourceDefault, false
	}
	if v := os.Getenv(pair.Mink); v != "" {
		return v, SourceMink, true
	}
	if v := os.Getenv(pair.Goose); v != "" {
		return v, SourceGoose, true
	}
	return "", SourceDefault, false
}

// resetForTesting resets the package-level singleton so unit tests can call Init again.
// Must only be called from test code.
func resetForTesting() {
	initOnce = sync.Once{}
	defaultPtr.Store(nil)
}
