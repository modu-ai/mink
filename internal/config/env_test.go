// Package config_test — Phase 2 envalias.Loader 통합 테스트.
// REQ-MINK-EM-004: MINK_* 우선, REQ-MINK-EM-005: GOOSE_* backward compat.
// TDD RED 단계에서 먼저 추가; envOverlay 가 envalias.Loader 를 사용하면 GREEN 이 된다.
package config_test

import (
	"testing"
	"testing/fstest"

	"github.com/modu-ai/mink/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// TestLoad_MinkEnvLocale는 MINK_LOCALE 단독 설정 시 cfg.UI.Locale 이 MINK 값으로
// 설정됨을 검증한다.
// REQ-MINK-EM-004: MINK_X 단독 설정 → 값 반영, deprecation warning 없음.
func TestLoad_MinkEnvLocale(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:       fstest.MapFS{},
		MinkHome: t.TempDir(),
		WorkDir:  t.TempDir(),
		EnvOverrides: map[string]string{
			"MINK_LOCALE": "ja",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "ja", cfg.UI.Locale)
	assert.Equal(t, config.SourceEnv, cfg.Source("ui.locale"))
}

// TestLoad_BothEnvLocale_PrefersMink는 MINK_LOCALE 과 GOOSE_LOCALE 이 동시에 설정되면
// MINK_LOCALE 값이 우선 적용되고, deprecation warning 은 출력되지 않음을 검증한다.
// REQ-MINK-EM-004: 두 키 충돌 시 MINK_X 우선, conflict warning (deprecation 아님) 1회.
func TestLoad_BothEnvLocale_PrefersMink(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	cfg, err := config.Load(config.LoadOptions{
		FS:       fstest.MapFS{},
		MinkHome: t.TempDir(),
		WorkDir:  t.TempDir(),
		Logger:   logger,
		EnvOverrides: map[string]string{
			"MINK_LOCALE":  "fr",
			"GOOSE_LOCALE": "ko",
		},
	})
	require.NoError(t, err)
	// MINK_LOCALE 이 우선
	assert.Equal(t, "fr", cfg.UI.Locale)
	// deprecation warning("deprecated env var") 는 없어야 한다.
	// (conflict warning 은 허용됨 — "both legacy and new env var set")
	for _, entry := range logs.All() {
		assert.NotContains(t, entry.Message, "deprecated env var",
			"MINK 우선 케이스에서 deprecation warning 이 출력되면 안 됨")
	}
}

// TestLoad_EnvOverlay_DeprecationWarningOnGooseOnly는 GOOSE_LOCALE 만 설정 시
// zaptest observer 로 deprecation warning 이 정확히 1회 출력됨을 검증한다.
// REQ-MINK-EM-004: GOOSE_X 단독 사용 시 "deprecated env var" warning 1회.
func TestLoad_EnvOverlay_DeprecationWarningOnGooseOnly(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	cfg, err := config.Load(config.LoadOptions{
		FS:       fstest.MapFS{},
		MinkHome: t.TempDir(),
		WorkDir:  t.TempDir(),
		Logger:   logger,
		EnvOverrides: map[string]string{
			"GOOSE_LOCALE": "ko",
		},
	})
	require.NoError(t, err)
	// backward compat: GOOSE_LOCALE 값이 반영되어야 한다 (REQ-MINK-EM-005)
	assert.Equal(t, "ko", cfg.UI.Locale)
	// deprecation warning 이 정확히 1회 출력되어야 한다
	deprecationLogs := logs.FilterMessage("deprecated env var, please rename")
	assert.Equal(t, 1, deprecationLogs.Len(),
		"GOOSE_LOCALE 단독 사용 시 deprecation warning 이 정확히 1회 출력되어야 함")
}
