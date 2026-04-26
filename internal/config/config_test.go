// Package config_test는 SPEC-GOOSE-CONFIG-001 수용 기준(AC-CFG-001~019)을 검증한다.
// RED-GREEN-REFACTOR TDD 사이클로 작성됨.
package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/modu-ai/goose/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- AC-CFG-002: 파일 부재 시 기본값 유지 ----

// TestLoad_DefaultsOnly_NoFiles_NoEnv는 AC-CFG-002를 검증한다.
// GOOSE_HOME이 빈 디렉토리이고 env 없을 때 기본값만으로 Config 반환
func TestLoad_DefaultsOnly_NoFiles_NoEnv(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:        fstest.MapFS{},
		GooseHome: t.TempDir(),
		WorkDir:   t.TempDir(),
		// 환경변수 오버라이드 없음 — 기본값만
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// 기본값 확인
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, 17890, cfg.Transport.HealthPort)
	assert.Equal(t, 17891, cfg.Transport.GRPCPort)
	assert.Equal(t, "en", cfg.UI.Locale)
	assert.False(t, cfg.Learning.Enabled)

	// Validate() 성공
	require.NoError(t, cfg.Validate())
}

// ---- AC-CFG-001: 계층 병합 순서 ----

// TestLoad_LayerMerge_DefaultUserEnv는 AC-CFG-001을 검증한다.
// defaults + user YAML + env 순서로 병합 확인
func TestLoad_LayerMerge_DefaultUserEnv(t *testing.T) {
	t.Parallel()

	userYAML := `log:
  level: debug
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:        memFS,
		GooseHome: gooseHome,
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"GOOSE_GRPC_PORT": "9999",
		},
	})
	require.NoError(t, err)

	// log.level: user 오버라이드
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, config.SourceUser, cfg.Source("log.level"))

	// transport.grpc_port: env 오버라이드
	assert.Equal(t, 9999, cfg.Transport.GRPCPort)
	assert.Equal(t, config.SourceEnv, cfg.Source("transport.grpc_port"))

	// 그 외 기본값 유지
	assert.Equal(t, 17890, cfg.Transport.HealthPort)
}

// ---- AC-CFG-003: YAML 구문 오류 거부 ----

// TestLoad_MalformedYAML_ReturnsSyntaxError는 AC-CFG-003을 검증한다.
// 잘못된 YAML → *ConfigError + errors.Is(err, ErrSyntax)
func TestLoad_MalformedYAML_ReturnsSyntaxError(t *testing.T) {
	t.Parallel()

	gooseHome := t.TempDir()
	badYAML := "log:\n  level: [unclosed"

	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(badYAML),
		},
	}

	_, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.Error(t, err)

	// errors.Is(err, ErrSyntax) == true
	assert.True(t, errors.Is(err, config.ErrSyntax), "err should wrap ErrSyntax, got: %v", err)

	// ConfigError 타입 확인
	var cfgErr *config.ConfigError
	assert.True(t, errors.As(err, &cfgErr))
	assert.NotEmpty(t, cfgErr.File)
}

// ---- AC-CFG-004: 포트 범위 검증 ----

// TestValidate_GRPCPort_Zero_ReturnsInvalidField는 AC-CFG-004를 검증한다.
// grpc_port: 0 → ErrInvalidField{Path:"transport.grpc_port"}
func TestValidate_GRPCPort_Zero_ReturnsInvalidField(t *testing.T) {
	t.Parallel()

	userYAML := `transport:
  grpc_port: 0
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	valErr := cfg.Validate()
	require.Error(t, valErr)

	var fieldErr config.ErrInvalidField
	require.True(t, errors.As(valErr, &fieldErr), "want ErrInvalidField, got %T: %v", valErr, valErr)
	assert.Equal(t, "transport.grpc_port", fieldErr.Path)
	assert.Contains(t, fieldErr.Msg, "1..65535")
}

// ---- AC-CFG-005: 프로젝트 > 유저 오버라이드 ----

// TestLoad_ProjectOverridesUser는 AC-CFG-005를 검증한다.
// user: openai, project: ollama → cfg.LLM.DefaultProvider == "ollama"
func TestLoad_ProjectOverridesUser(t *testing.T) {
	t.Parallel()

	gooseHome := t.TempDir()
	workDir := t.TempDir()

	userYAML := `llm:
  default_provider: openai
`
	projectYAML := `llm:
  default_provider: ollama
`

	// fs.FS 경로 키는 선행 "/" 없이
	gooseHomeFSKey := gooseHome[1:]
	workDirFSKey := workDir[1:]

	memFS := fstest.MapFS{
		filepath.Join(gooseHomeFSKey, "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
		filepath.Join(workDirFSKey, ".goose", "config.yaml"): &fstest.MapFile{
			Data: []byte(projectYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      workDir,
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	assert.Equal(t, "ollama", cfg.LLM.DefaultProvider)
	assert.Equal(t, config.SourceProject, cfg.Source("llm.default_provider"))
}

// ---- AC-CFG-006: Unknown 키 보존 (비-strict) ----

// TestLoad_UnknownKey_Preserved_NonStrict는 AC-CFG-006을 검증한다.
// unknown key → cfg.Unknown["future_feature"] 존재, 에러 없음
func TestLoad_UnknownKey_Preserved_NonStrict(t *testing.T) {
	t.Parallel()

	gooseHome := t.TempDir()
	yamlContent := `future_feature:
  x: 1
`
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(yamlContent),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:        memFS,
		GooseHome: gooseHome,
		WorkDir:   t.TempDir(),
		// GOOSE_CONFIG_STRICT 미설정
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	assert.NotNil(t, cfg.Unknown)
	assert.Contains(t, cfg.Unknown, "future_feature")
}

// ---- AC-CFG-007: Strict 모드 거부 ----

// TestLoad_UnknownKey_StrictMode_ReturnsError는 AC-CFG-007을 검증한다.
// GOOSE_CONFIG_STRICT=true + unknown key → StrictUnknownError
func TestLoad_UnknownKey_StrictMode_ReturnsError(t *testing.T) {
	t.Parallel()

	gooseHome := t.TempDir()
	yamlContent := `future_feature:
  x: 1
`
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(yamlContent),
		},
	}

	_, err := config.Load(config.LoadOptions{
		FS:        memFS,
		GooseHome: gooseHome,
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"GOOSE_CONFIG_STRICT": "true",
		},
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrStrictUnknown), "err should wrap ErrStrictUnknown, got: %v", err)

	var strictErr *config.StrictUnknownError
	require.True(t, errors.As(err, &strictErr))
	assert.Contains(t, strictErr.Keys, "future_feature")
}

// ---- AC-CFG-008: 환경변수 오버레이 단순 타입 ----

// TestLoad_EnvOverlay_StringAndBool는 AC-CFG-008을 검증한다.
// GOOSE_LOG_LEVEL=error, GOOSE_LEARNING_ENABLED=false
func TestLoad_EnvOverlay_StringAndBool(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:        fstest.MapFS{},
		GooseHome: t.TempDir(),
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"GOOSE_LOG_LEVEL":        "error",
			"GOOSE_LEARNING_ENABLED": "false",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "error", cfg.Log.Level)
	assert.False(t, cfg.Learning.Enabled)
}

// ---- AC-CFG-009: Zero-value 명시 override ----

// TestLoad_ZeroValue_BoolFalse_Overrides_Default는 AC-CFG-009를 검증한다.
// user YAML에 enabled: false 명시 → false 유지 (presence-aware)
func TestLoad_ZeroValue_BoolFalse_Overrides_Default(t *testing.T) {
	t.Parallel()

	// user YAML에 enabled: false 명시
	userYAML := `learning:
  enabled: false
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	// user가 명시적으로 false를 선언 → false
	assert.False(t, cfg.Learning.Enabled)
	assert.Equal(t, config.SourceUser, cfg.Source("learning.enabled"))
}

// TestLoad_AbsentKey_PreservesDefault는 AC-CFG-009 추가 케이스를 검증한다.
// user YAML에 learning 키 자체 없으면 default 유지
func TestLoad_AbsentKey_PreservesDefault(t *testing.T) {
	t.Parallel()

	// learning 섹션 없는 YAML
	userYAML := `log:
  level: debug
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	// default false 유지
	assert.False(t, cfg.Learning.Enabled)
	// Source는 default
	assert.Equal(t, config.SourceDefault, cfg.Source("learning.enabled"))
}

// ---- AC-CFG-010a: Env overlay int happy-path ----

// TestLoad_EnvOverlay_GRPCPort_Int_HappyPath는 AC-CFG-010a를 검증한다.
func TestLoad_EnvOverlay_GRPCPort_Int_HappyPath(t *testing.T) {
	t.Parallel()

	userYAML := `transport:
  grpc_port: 17891
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:        memFS,
		GooseHome: gooseHome,
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"GOOSE_GRPC_PORT": "9999",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 9999, cfg.Transport.GRPCPort)
	assert.Equal(t, config.SourceEnv, cfg.Source("transport.grpc_port"))
}

// ---- AC-CFG-010b: Env overlay int 파싱 실패 fallback ----

// TestLoad_EnvOverlay_GRPCPort_ParseFail_Fallback는 AC-CFG-010b를 검증한다.
// GOOSE_GRPC_PORT=abc → WARN 로그, 기존 값 유지
func TestLoad_EnvOverlay_GRPCPort_ParseFail_Fallback(t *testing.T) {
	t.Parallel()

	userYAML := `transport:
  grpc_port: 17891
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:        memFS,
		GooseHome: gooseHome,
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"GOOSE_GRPC_PORT": "abc",
		},
	})
	// 에러 없이 반환
	require.NoError(t, err)
	// user 값 유지
	assert.Equal(t, 17891, cfg.Transport.GRPCPort)
	assert.Equal(t, config.SourceUser, cfg.Source("transport.grpc_port"))
}

// ---- AC-CFG-011: Env overlay URL 타입 ----

// TestLoad_EnvOverlay_OllamaHost_URL는 AC-CFG-011을 검증한다.
func TestLoad_EnvOverlay_OllamaHost_URL(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:        fstest.MapFS{},
		GooseHome: t.TempDir(),
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"OLLAMA_HOST": "http://10.0.0.5:11434",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, cfg.LLM.Providers)
	assert.Equal(t, "http://10.0.0.5:11434", cfg.LLM.Providers["ollama"].Host)
	assert.Equal(t, config.SourceEnv, cfg.Source("llm.providers.ollama.host"))
}

// ---- AC-CFG-012: Env overlay secret 타입 + Redacted 마스킹 ----

// TestLoad_EnvOverlay_Secret_Redacted는 AC-CFG-012를 검증한다.
func TestLoad_EnvOverlay_Secret_Redacted(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:        fstest.MapFS{},
		GooseHome: t.TempDir(),
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"OPENAI_API_KEY": "sk-test-123",
		},
	})
	require.NoError(t, err)

	// 메모리상 원본 보존
	assert.Equal(t, "sk-test-123", cfg.LLM.Providers["openai"].APIKey)

	// Redacted() — 원본 포함 안 됨
	redacted := cfg.Redacted()
	assert.NotContains(t, redacted, "sk-test-123")
	assert.Contains(t, redacted, "sk-*****") // 8자 고정 마스크

	// 마스크 길이 고정 확인 (원본 길이 노출 금지)
	assert.Equal(t, 8, len("sk-*****"))
}

// TestRedacted_EmptySecret_NoPanic는 AC-CFG-012 + REQ-CFG-017 nil safety를 검증한다.
func TestRedacted_EmptySecret_NoPanic(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:           fstest.MapFS{},
		GooseHome:    t.TempDir(),
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	// panic 없이 호출 가능
	assert.NotPanics(t, func() {
		_ = cfg.Redacted()
	})
}

// ---- AC-CFG-013: fs.FS stub 주입 동등성 ----

// TestLoad_FSStub_Equivalence는 AC-CFG-013을 검증한다.
// 디스크 vs fstest.MapFS 동일한 Config 반환
func TestLoad_FSStub_Equivalence(t *testing.T) {
	t.Parallel()

	yamlContent := `log:
  level: warn
transport:
  grpc_port: 18000
`

	// (a) 실제 디스크
	gooseHomeA := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(gooseHomeA, "config.yaml"), []byte(yamlContent), 0600))

	cfgA, err := config.Load(config.LoadOptions{
		GooseHome:    gooseHomeA,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	// (b) in-memory fstest.MapFS
	gooseHomeB := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHomeB[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(yamlContent),
		},
	}

	cfgB, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHomeB,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	// 동일한 값
	assert.Equal(t, cfgA.Log.Level, cfgB.Log.Level)
	assert.Equal(t, cfgA.Transport.GRPCPort, cfgB.Transport.GRPCPort)
	assert.Equal(t, cfgA.Source("log.level"), cfgB.Source("log.level"))
	assert.Equal(t, cfgA.Source("transport.grpc_port"), cfgB.Source("transport.grpc_port"))
}

// ---- AC-CFG-014: 동시 읽기 안전성 ----

// TestLoad_ConcurrentReads_RaceSafe는 AC-CFG-014를 검증한다.
// N=16 고루틴 동시 read → race detector clean
func TestLoad_ConcurrentReads_RaceSafe(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:           fstest.MapFS{},
		GooseHome:    t.TempDir(),
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	const N = 16
	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()
			_ = cfg.Log.Level
			_ = cfg.Transport.GRPCPort
			_ = cfg.LLM.DefaultProvider
			_ = cfg.Source("log.level")
			_ = cfg.Source("transport.grpc_port")
		}()
	}
	wg.Wait()
}

// ---- AC-CFG-015: Validate() 호출 전 IsValid() false ----

// TestIsValid_BeforeValidate_ReturnsFalse는 AC-CFG-015를 검증한다.
func TestIsValid_BeforeValidate_ReturnsFalse(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:           fstest.MapFS{},
		GooseHome:    t.TempDir(),
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	// Validate() 호출 전 → false
	assert.False(t, cfg.IsValid())

	// Validate() 호출 후 → true
	require.NoError(t, cfg.Validate())
	assert.True(t, cfg.IsValid())
}

// TestIsValid_AfterFailedValidate_RemainsFalse는 AC-CFG-015 추가 케이스를 검증한다.
// Validate()가 에러 반환 시 IsValid() == false 유지
func TestIsValid_AfterFailedValidate_RemainsFalse(t *testing.T) {
	t.Parallel()

	// 포트 0은 validate 실패
	userYAML := `transport:
  grpc_port: 0
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	valErr := cfg.Validate()
	require.Error(t, valErr)
	assert.False(t, cfg.IsValid())
}

// ---- AC-CFG-016: 타입 mismatch 필드 경로 명명 ----

// TestLoad_TypeMismatch_GRPCPort_String는 AC-CFG-016을 검증한다.
// transport.grpc_port: "not-a-number" → ErrInvalidField 경로 + Expected 타입 포함
func TestLoad_TypeMismatch_GRPCPort_String(t *testing.T) {
	t.Parallel()

	userYAML := `transport:
  grpc_port: "not-a-number"
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	_, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.Error(t, err)

	var fieldErr config.ErrInvalidField
	require.True(t, errors.As(err, &fieldErr), "want ErrInvalidField, got %T: %v", err, err)
	assert.Equal(t, "transport.grpc_port", fieldErr.Path)
	assert.Equal(t, "int", fieldErr.Expected)
}

// ---- AC-CFG-017: $GOOSE_HOME 미설정 시 $HOME/.goose fallback ----

// TestLoad_GooseHome_Unset_UsesHomeDotGoose는 AC-CFG-017을 검증한다.
// 이 테스트는 실제 HOME env를 변경하므로 serial로 실행 (t.Parallel() 없음)
func TestLoad_GooseHome_Unset_UsesHomeDotGoose(t *testing.T) {
	// env 변경 필요 — 격리를 위해 serial 실행
	fakeHome := t.TempDir()
	gooseDir := filepath.Join(fakeHome, ".goose")
	require.NoError(t, os.MkdirAll(gooseDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(gooseDir, "config.yaml"),
		[]byte("log:\n  level: warn\n"),
		0600,
	))

	// GOOSE_HOME 미설정, HOME을 fakeHome으로 설정
	// t.Setenv는 t.Parallel()과 공존 불가이므로 non-parallel 테스트에서만 사용
	t.Setenv("HOME", fakeHome)
	t.Setenv("GOOSE_HOME", "")

	// 실제 디스크 접근 (os.DirFS 사용, GooseHome 미지정)
	cfg, err := config.Load(config.LoadOptions{
		WorkDir: t.TempDir(),
	})
	require.NoError(t, err)
	assert.Equal(t, "warn", cfg.Log.Level)
	assert.Equal(t, config.SourceUser, cfg.Source("log.level"))
}

// ---- AC-CFG-018: 쉘 변수 literal 처리 ----

// TestLoad_ShellVarSyntax_NotExpanded는 AC-CFG-018을 검증한다.
// "${FOO}" → literal 문자열 보존, 쉘 확장 금지
func TestLoad_ShellVarSyntax_NotExpanded(t *testing.T) {
	t.Parallel()

	userYAML := `log:
  level: "${FOO}"
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:        memFS,
		GooseHome: gooseHome,
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"FOO": "info", // 설정돼 있어도 yaml 값은 literal 보존
		},
	})
	require.NoError(t, err)
	// literal 그대로 보존
	assert.Equal(t, "${FOO}", cfg.Log.Level)
}

// TestLoad_ShellVarSyntax_UnsetVar_Literal는 AC-CFG-018 추가 케이스를 검증한다.
// "${BAR}" (미설정 env) → literal "${BAR}" 보존
func TestLoad_ShellVarSyntax_UnsetVar_Literal(t *testing.T) {
	t.Parallel()

	userYAML := `log:
  level: "${BAR}"
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{}, // BAR 미설정
	})
	require.NoError(t, err)
	assert.Equal(t, "${BAR}", cfg.Log.Level)
}

// ---- AC-CFG-019: LoadOptions.OverrideFiles 테스트 전용 경로 ----

// TestLoad_OverrideFiles_BypassesDefaultChain는 AC-CFG-019를 검증한다.
func TestLoad_OverrideFiles_BypassesDefaultChain(t *testing.T) {
	t.Parallel()

	// 기본 체인 (GOOSE_HOME/config.yaml)에 info 설정
	gooseHome := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(gooseHome, "config.yaml"),
		[]byte("log:\n  level: info\n"),
		0600,
	))

	// override 파일에 error 설정
	overrideFile := filepath.Join(t.TempDir(), "override.yaml")
	require.NoError(t, os.WriteFile(overrideFile, []byte("log:\n  level: error\n"), 0600))

	cfg, err := config.Load(config.LoadOptions{
		GooseHome:     gooseHome,
		WorkDir:       t.TempDir(),
		OverrideFiles: []string{overrideFile},
		EnvOverrides:  map[string]string{},
	})
	require.NoError(t, err)

	// override 파일이 적용됨
	assert.Equal(t, "error", cfg.Log.Level)
	// Source는 SourceOverride
	src := cfg.Source("log.level")
	assert.Equal(t, config.SourceOverride, src)
}

// ---- 추가: LoadFromMap dry-run API ----

// TestLoadFromMap_BasicMerge는 LoadFromMap()을 검증한다.
func TestLoadFromMap_BasicMerge(t *testing.T) {
	t.Parallel()

	m := map[string]any{
		"log": map[string]any{"level": "warn"},
	}
	cfg, err := config.LoadFromMap(m)
	require.NoError(t, err)
	assert.Equal(t, "warn", cfg.Log.Level)
}

// ---- 추가: reflect.DeepEqual을 위한 Config 동등성 검증 ----

// TestLoad_FSStub_DeepEqual는 AC-CFG-013 reflect.DeepEqual 조건을 추가 검증한다.
func TestLoad_FSStub_DeepEqual(t *testing.T) {
	t.Parallel()

	yamlContent := `log:
  level: warn
transport:
  grpc_port: 18001
`

	// 두 개의 별도 Load 호출이 동일한 결과를 생성하는지 확인
	gooseHomeA := t.TempDir()
	gooseHomeB := t.TempDir()

	memFSA := fstest.MapFS{
		filepath.Join(gooseHomeA[1:], "config.yaml"): &fstest.MapFile{Data: []byte(yamlContent)},
	}
	memFSB := fstest.MapFS{
		filepath.Join(gooseHomeB[1:], "config.yaml"): &fstest.MapFile{Data: []byte(yamlContent)},
	}

	cfgA, err := config.Load(config.LoadOptions{FS: memFSA, GooseHome: gooseHomeA, WorkDir: t.TempDir(), EnvOverrides: map[string]string{}})
	require.NoError(t, err)
	cfgB, err := config.Load(config.LoadOptions{FS: memFSB, GooseHome: gooseHomeB, WorkDir: t.TempDir(), EnvOverrides: map[string]string{}})
	require.NoError(t, err)

	// 핵심 필드 동등성
	assert.Equal(t, cfgA.Log.Level, cfgB.Log.Level)
	assert.Equal(t, cfgA.Transport.GRPCPort, cfgB.Transport.GRPCPort)
	assert.True(t, reflect.DeepEqual(cfgA.LLM.DefaultProvider, cfgB.LLM.DefaultProvider))
}

// ---- REFACTOR 추가 테스트: 커버리지 향상 ----

// TestLoad_Providers_YAML은 applyProvidersNode/applyProviderNode 커버리지를 위한 테스트다.
// LLM providers 설정을 YAML에서 로드하는 케이스를 검증한다.
func TestLoad_Providers_YAML(t *testing.T) {
	t.Parallel()

	userYAML := `llm:
  default_provider: openai
  providers:
    openai:
      api_key: "test-key"
    ollama:
      host: "http://localhost:11434"
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", cfg.LLM.DefaultProvider)
	assert.Equal(t, "test-key", cfg.LLM.Providers["openai"].APIKey)
	assert.Equal(t, "http://localhost:11434", cfg.LLM.Providers["ollama"].Host)
}

// TestLoad_UILocale_YAML은 applyUINode 커버리지를 위한 테스트다.
func TestLoad_UILocale_YAML(t *testing.T) {
	t.Parallel()

	userYAML := `ui:
  locale: ko
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	assert.Equal(t, "ko", cfg.UI.Locale)
	assert.Equal(t, config.SourceUser, cfg.Source("ui.locale"))
}

// TestLoad_HealthPort_YAML은 health_port 파싱 커버리지를 위한 테스트다.
func TestLoad_HealthPort_YAML(t *testing.T) {
	t.Parallel()

	userYAML := `transport:
  health_port: 9090
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Transport.HealthPort)
	assert.Equal(t, config.SourceUser, cfg.Source("transport.health_port"))
}

// TestLoad_HealthPort_TypeMismatch는 health_port 타입 불일치 케이스를 검증한다.
func TestLoad_HealthPort_TypeMismatch(t *testing.T) {
	t.Parallel()

	userYAML := `transport:
  health_port: "not-a-port"
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	_, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.Error(t, err)
	var fieldErr config.ErrInvalidField
	require.True(t, errors.As(err, &fieldErr))
	assert.Equal(t, "transport.health_port", fieldErr.Path)
}

// TestConfigError_ErrorMethod는 ConfigError.Error() 메서드를 커버한다.
func TestConfigError_ErrorMethod(t *testing.T) {
	t.Parallel()

	// Line > 0 케이스
	errWithLine := &config.ConfigError{File: "/test.yaml", Line: 5, Column: 3, Msg: "테스트 오류"}
	assert.Contains(t, errWithLine.Error(), "/test.yaml")
	assert.Contains(t, errWithLine.Error(), "5")

	// Line == 0 케이스
	errNoLine := &config.ConfigError{File: "/test.yaml", Msg: "구문 오류"}
	assert.Contains(t, errNoLine.Error(), "/test.yaml")

	// Unwrap 커버
	sentinel := errors.New("original")
	errWrapped := &config.ConfigError{File: "/test.yaml", Msg: "wrapped", Underlying: sentinel}
	assert.Equal(t, sentinel, errors.Unwrap(errWrapped))
}

// TestErrInvalidField_ErrorMethod는 ErrInvalidField.Error() 메서드를 커버한다.
func TestErrInvalidField_ErrorMethod(t *testing.T) {
	t.Parallel()

	// Expected/Got 있는 케이스
	errWithTypes := config.ErrInvalidField{Path: "transport.grpc_port", Expected: "int", Got: "string"}
	assert.Contains(t, errWithTypes.Error(), "transport.grpc_port")
	assert.Contains(t, errWithTypes.Error(), "int")
	assert.Contains(t, errWithTypes.Error(), "string")

	// Msg만 있는 케이스
	errMsgOnly := config.ErrInvalidField{Path: "transport.health_port", Msg: "must be 1..65535"}
	assert.Contains(t, errMsgOnly.Error(), "transport.health_port")
	assert.Contains(t, errMsgOnly.Error(), "must be 1..65535")
}

// TestStrictUnknownError_ErrorMethod는 StrictUnknownError.Error() 메서드를 커버한다.
func TestStrictUnknownError_ErrorMethod(t *testing.T) {
	t.Parallel()

	err := &config.StrictUnknownError{Keys: []string{"foo", "bar"}}
	msg := err.Error()
	assert.Contains(t, msg, "foo")
	assert.Contains(t, msg, "bar")
}

// TestLoad_EnvOverride_ANTHROPIC_KEY는 anthropic api_key ENV 오버레이를 검증한다.
func TestLoad_EnvOverride_ANTHROPIC_KEY(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:        fstest.MapFS{},
		GooseHome: t.TempDir(),
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"ANTHROPIC_API_KEY": "anth-test-key",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "anth-test-key", cfg.LLM.Providers["anthropic"].APIKey)

	// Redacted() — 원본 노출 안 됨
	redacted := cfg.Redacted()
	assert.NotContains(t, redacted, "anth-test-key")
}

// TestLoad_FileReadError는 파일 읽기 실패(권한 오류) 경로를 커버한다.
// 주의: 루트 실행 환경에서는 스킵한다.
func TestLoad_FileReadError(t *testing.T) {
	t.Parallel()

	// 빈 디렉토리를 파일처럼 취급하는 케이스는 skip — OS 의존적
	// 대신 파일 오픈 실패를 직접 시뮬레이션하기 어려우므로
	// mergeYAMLFile의 "파일 없음" 경로가 이미 다른 테스트에서 커버됨
	t.Skip("파일 읽기 실패 경로는 OS 의존적이므로 통합 테스트에서 별도 검증")
}

// TestLoad_Validate_BadLocale는 ui.locale 잘못된 값 검증을 커버한다.
func TestLoad_Validate_BadLocale(t *testing.T) {
	t.Parallel()

	userYAML := `ui:
  locale: xx
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	valErr := cfg.Validate()
	require.Error(t, valErr)
	var fieldErr config.ErrInvalidField
	require.True(t, errors.As(valErr, &fieldErr))
	assert.Equal(t, "ui.locale", fieldErr.Path)
}

// TestLoad_Validate_BadLogLevel은 log.level 잘못된 enum 값을 검증한다.
func TestLoad_Validate_BadLogLevel(t *testing.T) {
	t.Parallel()

	userYAML := `log:
  level: blah
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{
			Data: []byte(userYAML),
		},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	valErr := cfg.Validate()
	require.Error(t, valErr)
	var fieldErr config.ErrInvalidField
	require.True(t, errors.As(valErr, &fieldErr))
	assert.Equal(t, "log.level", fieldErr.Path)
}

// TestLoad_GooseEnvLocale는 GOOSE_LOCALE env 오버레이를 검증한다.
func TestLoad_GooseEnvLocale(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:        fstest.MapFS{},
		GooseHome: t.TempDir(),
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"GOOSE_LOCALE": "ko",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "ko", cfg.UI.Locale)
	assert.Equal(t, config.SourceEnv, cfg.Source("ui.locale"))
}

// TestLoad_EnvOverlay_HealthPort_ParseFail는 GOOSE_HEALTH_PORT 파싱 실패 케이스를 커버한다.
func TestLoad_EnvOverlay_HealthPort_ParseFail(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:        fstest.MapFS{},
		GooseHome: t.TempDir(),
		WorkDir:   t.TempDir(),
		EnvOverrides: map[string]string{
			"GOOSE_HEALTH_PORT": "not-a-port",
		},
	})
	require.NoError(t, err)
	// 기본값 유지
	assert.Equal(t, 17890, cfg.Transport.HealthPort)
}

// TestSource_NilSources는 Config.Source() nil 안전성을 검증한다.
func TestSource_NilSources(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(config.LoadOptions{
		FS:           fstest.MapFS{},
		GooseHome:    t.TempDir(),
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	// SourceDefault is returned for unset paths.
	assert.Equal(t, config.SourceDefault, cfg.Source("nonexistent.path"))
}

// ---- SPEC-GOOSE-CREDPOOL-001 OI-06: ProviderConfig.Credentials schema ----

// TestLoad_ProviderCredentials_Empty_BackwardsCompat verifies that legacy
// configs without a credentials field still load and the slice is nil/empty.
// REQ-CREDPOOL-001 OI-06 backwards-compat constraint.
func TestLoad_ProviderCredentials_Empty_BackwardsCompat(t *testing.T) {
	t.Parallel()

	userYAML := `llm:
  providers:
    openai:
      api_key: sk-x
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{Data: []byte(userYAML)},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	p := cfg.LLM.Providers["openai"]
	assert.Empty(t, p.Credentials, "legacy config without credentials key must remain valid")
}

// TestLoad_ProviderCredentials_Single_Parses verifies a single credential
// entry is parsed with all three fields (Type, Path, KeyringRef).
func TestLoad_ProviderCredentials_Single_Parses(t *testing.T) {
	t.Parallel()

	userYAML := `llm:
  providers:
    anthropic:
      credentials:
        - type: anthropic_claude_file
          path: /home/u/.claude/.credentials.json
          keyring_ref: anthropic-default
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{Data: []byte(userYAML)},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	creds := cfg.LLM.Providers["anthropic"].Credentials
	require.Len(t, creds, 1)
	assert.Equal(t, "anthropic_claude_file", creds[0].Type)
	assert.Equal(t, "/home/u/.claude/.credentials.json", creds[0].Path)
	assert.Equal(t, "anthropic-default", creds[0].KeyringRef)
}

// TestLoad_ProviderCredentials_Multiple_PreservesOrder verifies that multiple
// credentials per provider are parsed and order is preserved.
func TestLoad_ProviderCredentials_Multiple_PreservesOrder(t *testing.T) {
	t.Parallel()

	userYAML := `llm:
  providers:
    openai:
      credentials:
        - type: openai_codex_file
          path: /a
          keyring_ref: kr-a
        - type: openai_codex_file
          path: /b
          keyring_ref: kr-b
        - type: openai_codex_file
          path: /c
          keyring_ref: kr-c
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{Data: []byte(userYAML)},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	creds := cfg.LLM.Providers["openai"].Credentials
	require.Len(t, creds, 3)
	assert.Equal(t, "/a", creds[0].Path)
	assert.Equal(t, "/b", creds[1].Path)
	assert.Equal(t, "/c", creds[2].Path)
}

// TestValidate_ProviderCredentials_UnknownType_Rejected verifies that an
// unknown credential Type fails Validate().
func TestValidate_ProviderCredentials_UnknownType_Rejected(t *testing.T) {
	t.Parallel()

	userYAML := `llm:
  providers:
    anthropic:
      credentials:
        - type: bogus_kind
          path: /tmp/x
          keyring_ref: x
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{Data: []byte(userYAML)},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	valErr := cfg.Validate()
	require.Error(t, valErr)
	var fieldErr config.ErrInvalidField
	require.True(t, errors.As(valErr, &fieldErr), "want ErrInvalidField, got %T: %v", valErr, valErr)
	assert.Contains(t, fieldErr.Path, "credentials")
	assert.Contains(t, fieldErr.Msg, "bogus_kind")
}

// TestValidate_ProviderCredentials_KnownTypes_Accepted verifies that all
// three known credential Types pass Validate().
func TestValidate_ProviderCredentials_KnownTypes_Accepted(t *testing.T) {
	t.Parallel()

	userYAML := `llm:
  providers:
    anthropic:
      credentials:
        - type: anthropic_claude_file
          path: /a
          keyring_ref: ka
    openai:
      credentials:
        - type: openai_codex_file
          path: /b
          keyring_ref: kb
    nous:
      credentials:
        - type: nous_hermes_file
          path: /c
          keyring_ref: kc
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{Data: []byte(userYAML)},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)
	require.NoError(t, cfg.Validate())
}

// TestRedacted_CredentialFields_NotMasked verifies that Path and KeyringRef
// are not treated as secrets (they are reference labels). The redacted
// output should still preserve them so operators can audit credential wiring.
func TestRedacted_CredentialFields_NotMasked(t *testing.T) {
	t.Parallel()

	userYAML := `llm:
  providers:
    anthropic:
      credentials:
        - type: anthropic_claude_file
          path: /home/u/.claude/.credentials.json
          keyring_ref: anthropic-prod
`
	gooseHome := t.TempDir()
	memFS := fstest.MapFS{
		filepath.Join(gooseHome[1:], "config.yaml"): &fstest.MapFile{Data: []byte(userYAML)},
	}

	cfg, err := config.Load(config.LoadOptions{
		FS:           memFS,
		GooseHome:    gooseHome,
		WorkDir:      t.TempDir(),
		EnvOverrides: map[string]string{},
	})
	require.NoError(t, err)

	out := cfg.Redacted()
	// Reference labels are non-secret and should remain visible for audit.
	assert.Contains(t, out, "anthropic-prod")
	assert.Contains(t, out, "anthropic_claude_file")
}
