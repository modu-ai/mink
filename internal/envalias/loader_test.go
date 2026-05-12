package envalias_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/modu-ai/mink/internal/envalias"
)

// makeObserverLogger returns a zap logger backed by a log observer, and the observer itself.
func makeObserverLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.WarnLevel)
	return zap.New(core), logs
}

// makeStaticLookup returns an EnvLookup func that resolves keys from a static map.
func makeStaticLookup(env map[string]string) func(string) string {
	return func(key string) string {
		return env[key]
	}
}

// TestEnvSourceString confirms the String() method covers all three values.
func TestEnvSourceString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		src  envalias.EnvSource
		want string
	}{
		{envalias.SourceDefault, "default"},
		{envalias.SourceMink, "mink"},
		{envalias.SourceGoose, "goose"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.src.String(), "EnvSource(%d).String()", int(tc.src))
	}
}

// TestNew_DefaultsEnvLookupToOsGetenv confirms that a nil EnvLookup falls back to os.Getenv
// without panicking. We only verify the loader can be constructed and called.
func TestNew_DefaultsEnvLookupToOsGetenv(t *testing.T) {
	t.Parallel()

	logger, _ := makeObserverLogger()
	l := envalias.New(envalias.Options{Logger: logger}) // EnvLookup intentionally nil

	// "HOME" is a registered key. Real os.Getenv will be called; value is not asserted
	// because CI environments vary. The test validates no panic occurs.
	_, _, _ = l.Get("HOME")
}

// TestGet_MinkOnly confirms that when only MINK_X is set, value is returned from SourceMink
// and no deprecation warning is emitted.
func TestGet_MinkOnly(t *testing.T) {
	t.Parallel()

	logger, logs := makeObserverLogger()
	env := map[string]string{"MINK_LOG_LEVEL": "debug"}
	l := envalias.New(envalias.Options{Logger: logger, EnvLookup: makeStaticLookup(env)})

	value, source, ok := l.Get("LOG_LEVEL")

	require.True(t, ok)
	assert.Equal(t, "debug", value)
	assert.Equal(t, envalias.SourceMink, source)
	assert.Equal(t, 0, logs.Len(), "no warning expected when only MINK_X is set")
}

// TestGet_GooseOnly confirms that when only GOOSE_X is set, value is returned from SourceGoose
// and a deprecation warning is emitted exactly once.
func TestGet_GooseOnly(t *testing.T) {
	t.Parallel()

	logger, logs := makeObserverLogger()
	env := map[string]string{"GOOSE_LOG_LEVEL": "warn"}
	l := envalias.New(envalias.Options{Logger: logger, EnvLookup: makeStaticLookup(env)})

	// First call — should warn.
	value, source, ok := l.Get("LOG_LEVEL")
	require.True(t, ok)
	assert.Equal(t, "warn", value)
	assert.Equal(t, envalias.SourceGoose, source)
	require.Equal(t, 1, logs.Len(), "deprecation warning must be emitted on first call")
	assert.Equal(t, "deprecated env var, please rename", logs.All()[0].Message)
	assert.Equal(t, "GOOSE_LOG_LEVEL", logs.All()[0].ContextMap()["old"])
	assert.Equal(t, "MINK_LOG_LEVEL", logs.All()[0].ContextMap()["new"])

	// Second call — sync.Once must suppress duplicate warning.
	_, _, _ = l.Get("LOG_LEVEL")
	assert.Equal(t, 1, logs.Len(), "deprecation warning must fire at most once per key per loader instance")
}

// TestGet_BothSet confirms that when both MINK_X and GOOSE_X are set, MINK_X wins
// and a conflict (not deprecation) warning is emitted once.
func TestGet_BothSet(t *testing.T) {
	t.Parallel()

	logger, logs := makeObserverLogger()
	env := map[string]string{
		"MINK_LOG_LEVEL":  "info",
		"GOOSE_LOG_LEVEL": "debug",
	}
	l := envalias.New(envalias.Options{Logger: logger, EnvLookup: makeStaticLookup(env)})

	value, source, ok := l.Get("LOG_LEVEL")
	require.True(t, ok)
	assert.Equal(t, "info", value, "MINK_X must win when both are set")
	assert.Equal(t, envalias.SourceMink, source)
	require.Equal(t, 1, logs.Len(), "conflict warning must be emitted once")
	assert.Equal(t, "both legacy and new env var set; using new key", logs.All()[0].Message)

	// Second call — no additional warning.
	_, _, _ = l.Get("LOG_LEVEL")
	assert.Equal(t, 1, logs.Len(), "conflict warning must fire at most once per key per loader instance")
}

// TestGet_NeitherSet confirms that when neither MINK_X nor GOOSE_X is set, the loader returns
// empty value with SourceDefault and ok=false.
func TestGet_NeitherSet(t *testing.T) {
	t.Parallel()

	logger, logs := makeObserverLogger()
	l := envalias.New(envalias.Options{Logger: logger, EnvLookup: makeStaticLookup(nil)})

	value, source, ok := l.Get("LOG_LEVEL")

	assert.False(t, ok)
	assert.Empty(t, value)
	assert.Equal(t, envalias.SourceDefault, source)
	assert.Equal(t, 0, logs.Len(), "no warning when neither key is set")
}

// TestGet_UnregisteredKey confirms that an unregistered key returns SourceDefault/ok=false
// without logging when StrictMode is false (default).
func TestGet_UnregisteredKey_ReturnsDefault(t *testing.T) {
	t.Parallel()

	logger, logs := makeObserverLogger()
	l := envalias.New(envalias.Options{Logger: logger, EnvLookup: makeStaticLookup(nil)})

	value, source, ok := l.Get("COMPLETELY_UNKNOWN_KEY")

	assert.False(t, ok)
	assert.Empty(t, value)
	assert.Equal(t, envalias.SourceDefault, source)
	assert.Equal(t, 0, logs.Len(), "no log when strict mode is off and key is unregistered")
}

// TestStrictMode_UnknownKey_Logs confirms that an unregistered key emits a warning log
// when StrictMode is true, but still returns SourceDefault/ok=false.
func TestStrictMode_UnknownKey_Logs(t *testing.T) {
	t.Parallel()

	logger, logs := makeObserverLogger()
	l := envalias.New(envalias.Options{
		Logger:     logger,
		EnvLookup:  makeStaticLookup(nil),
		StrictMode: true,
	})

	value, source, ok := l.Get("COMPLETELY_UNKNOWN_KEY_STRICT")

	assert.False(t, ok)
	assert.Empty(t, value)
	assert.Equal(t, envalias.SourceDefault, source)
	require.Equal(t, 1, logs.Len(), "strict mode must emit a warning for unregistered key")
	assert.Equal(t, "envalias.Get called with unregistered key", logs.All()[0].Message)
}

// TestAllKeysRegistered verifies that all 21 single-key entries from spec.md §7.3 are present
// in the loader mapping table. This is a compile-time table completeness check.
func TestAllKeysRegistered(t *testing.T) {
	t.Parallel()

	// canonical 21 short keys from spec.md §7.3 (GOOSE_AUTH_* prefix handled separately)
	wantKeys := []string{
		"HOME",
		"LOG_LEVEL",
		"HEALTH_PORT",
		"GRPC_PORT",
		"LOCALE",
		"LEARNING_ENABLED",
		"CONFIG_STRICT",
		"GRPC_REFLECTION",
		"GRPC_MAX_RECV_MSG_BYTES",
		"SHUTDOWN_TOKEN",
		"HOOK_TRACE",
		"HOOK_NON_INTERACTIVE",
		"ALIAS_STRICT",
		"QWEN_REGION",
		"KIMI_REGION",
		"TELEGRAM_BOT_TOKEN",
		"AUTH_TOKEN",
		"AUTH_REFRESH",
		"HISTORY_SNIP",
		"METRICS_ENABLED",
		"GRPC_BIND",
	}

	logger, _ := makeObserverLogger()
	l := envalias.New(envalias.Options{Logger: logger, EnvLookup: makeStaticLookup(nil)})

	for _, key := range wantKeys {
		// We distinguish "key not registered" from "key registered but value empty"
		// by using StrictMode — if the key is unregistered, strict mode would log a warning.
		// Here we use a separate loader with StrictMode to detect missing registrations.
		strictLogger, strictLogs := makeObserverLogger()
		strictLoader := envalias.New(envalias.Options{
			Logger:     strictLogger,
			EnvLookup:  makeStaticLookup(nil),
			StrictMode: true,
		})

		_, _, _ = strictLoader.Get(key)
		assert.Equal(t, 0, strictLogs.Len(),
			"key %q must be registered in keyMappings (strict mode emitted a warning — key is missing)", key)

		_ = l // ensure non-strict loader compiles
	}
}

// TestMinkPrefixes confirms that MinkPrefixes returns the known prefix strings used by
// the deny-list helper for internal/hook consumption (Phase 4 preparation).
func TestMinkPrefixes(t *testing.T) {
	t.Parallel()

	prefixes := envalias.MinkPrefixes()
	require.NotEmpty(t, prefixes, "MinkPrefixes must return at least one prefix")

	found := false
	for _, p := range prefixes {
		if p == "MINK_AUTH_" {
			found = true
		}
	}
	assert.True(t, found, "MinkPrefixes must include MINK_AUTH_ for deny-list use")
}
