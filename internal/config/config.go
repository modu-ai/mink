// Package config는 goosed 계층형 설정 로더를 제공한다.
// SPEC-GOOSE-CONFIG-001 — 계층형 설정 로더
//
// 로딩 우선순위 (낮음 → 높음):
//
//	defaults → project(YAML) → user(YAML) → runtime(env)
//
// @MX:ANCHOR: [AUTO] 모든 goosed 부트스트랩 + 후속 SPEC consumer가 호출하는 단일 진입점
// @MX:REASON: Load()는 TRANSPORT/LLM/AGENT/CLI 등 모든 후속 SPEC의 시작점이므로 fan_in >= 5 예상
// @MX:SPEC: SPEC-GOOSE-CONFIG-001 REQ-CFG-001
package config

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// Config는 goosed 전체 설정 구조체다.
// 불변(immutable) — Load() 반환 후 필드 변경 금지.
// REQ-CFG-003: concurrent reads safe (effectively frozen pointer)
type Config struct {
	Log       LogConfig       `yaml:"log"`
	Transport TransportConfig `yaml:"transport"`
	LLM       LLMConfig       `yaml:"llm"`
	Learning  LearningConfig  `yaml:"learning"`
	UI        UIConfig        `yaml:"ui"`
	Audit     AuditConfig     `yaml:"audit"`
	FSAccess  FSAccessConfig  `yaml:"fs_access"`
	// SkillsRoot는 skill SKILL.md 파일을 탐색할 루트 디렉토리다.
	// 빈 문자열이면 Load() 시 GOOSE_HOME/skills 로 설정된다.
	// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002 step 7
	SkillsRoot string `yaml:"skills_root"`
	// Unknown은 최상위 알 수 없는 키를 보존한다.
	// REQ-CFG-008: unknown top-level keys 보존
	Unknown map[string]any `yaml:",inline"`

	// 내부 상태 — 외부에서 직접 접근 금지
	sources   sourceMap
	validated bool
}

// LogConfig는 로그 설정이다.
type LogConfig struct {
	// Level은 로그 레벨이다 (debug|info|warn|error).
	Level string `yaml:"level"`
}

// TransportConfig는 전송 계층 설정이다.
type TransportConfig struct {
	// HealthPort는 헬스체크 서버 포트다. 기본값: 0 (disabled).
	HealthPort int `yaml:"health_port"`
	// GRPCPort는 gRPC 서버 포트다. 기본값: 9005.
	GRPCPort int `yaml:"grpc_port"`
}

// LLMConfig는 LLM 설정이다.
type LLMConfig struct {
	// DefaultProvider는 기본 LLM 프로바이더 이름이다.
	DefaultProvider string `yaml:"default_provider"`
	// Providers는 프로바이더별 설정 맵이다.
	Providers map[string]ProviderConfig `yaml:"providers"`
}

// ProviderConfig is the per-provider LLM configuration.
type ProviderConfig struct {
	// Host is the provider host URL (for example, ollama).
	Host string `yaml:"host"`
	// APIKey is the provider API key (secret-typed).
	// REQ-CFG-016: forward-reference hook for the future credential pool.
	APIKey string `yaml:"api_key"`
	// Credentials enumerates pooled credential sources for this provider.
	// SPEC-GOOSE-CREDPOOL-001 OI-06: consumed by credential.NewPoolsFromConfig
	// to build per-provider credential pools. An empty slice keeps existing
	// configs valid (backwards compatibility).
	Credentials []CredentialConfig `yaml:"credentials"`
}

// CredentialConfig describes a single credential source entry inside a
// provider's pool. SPEC-GOOSE-CREDPOOL-001 OI-06.
//
// @MX:NOTE: [AUTO] Schema for ProviderConfig.Credentials. The Type enum
// is validated by Config.Validate(); see credential.NewPoolsFromConfig
// for the matching factory dispatch table.
// @MX:SPEC: SPEC-GOOSE-CREDPOOL-001 OI-06
type CredentialConfig struct {
	// Type identifies the credential source kind. Known values:
	//   "anthropic_claude_file" — ~/.claude/.credentials.json reader
	//   "openai_codex_file"     — ~/.codex/auth.json reader
	//   "nous_hermes_file"      — ~/.hermes/auth.json reader
	// Future kinds (env, OS keyring, vault) extend this enum.
	Type string `yaml:"type"`
	// Path is the absolute filesystem path to the vendor credential file.
	// An empty string means "use the default path for this Type" — resolution
	// is delegated to the source factory.
	Path string `yaml:"path"`
	// KeyringRef is the opaque identifier the future credential proxy
	// (SPEC-GOOSE-CREDENTIAL-PROXY-001) will use to look up the actual secret
	// material. Source implementations forward this verbatim into
	// PooledCredential.KeyringID.
	KeyringRef string `yaml:"keyring_ref"`
}

// knownCredentialTypes lists every accepted CredentialConfig.Type value.
// SPEC-GOOSE-CREDPOOL-001 OI-06.
var knownCredentialTypes = map[string]struct{}{
	"anthropic_claude_file": {},
	"openai_codex_file":     {},
	"nous_hermes_file":      {},
}

// IsKnownCredentialType reports whether t is a recognized credential source
// kind. The credential factory uses the same table for dispatch.
func IsKnownCredentialType(t string) bool {
	_, ok := knownCredentialTypes[t]
	return ok
}

// LearningConfig는 학습 기능 설정이다.
type LearningConfig struct {
	// Enabled는 학습 기능 활성화 여부다. Phase 0 기본값: false.
	Enabled bool `yaml:"enabled"`
}

// UIConfig는 UI 설정이다.
type UIConfig struct {
	// Locale은 UI 언어 코드다 (en|ko|ja|zh).
	Locale string `yaml:"locale"`
}

// AuditConfig는 감사 로그 설정이다.
// SPEC-GOOSE-AUDIT-001 — Append-Only Audit Log
type AuditConfig struct {
	// Enabled는 감사 로그 활성화 여부다. 기본값: true.
	Enabled bool `yaml:"enabled"`
	// MaxSizeMB는 로테이션 전 최대 로그 파일 크기(MB)다. 기본값: 100.
	MaxSizeMB int `yaml:"max_size_mb"`
	// GlobalDir은 전역 감사 로그 디렉토리 경로다. 기본값: ~/.goose/logs.
	GlobalDir string `yaml:"global_dir"`
	// LocalDir은 프로젝트 감사 로그 디렉토리 경로다. 기본값: ./.goose/logs.
	LocalDir string `yaml:"local_dir"`
}

// LoadOptions는 Load() 동작을 제어하는 옵션이다.
type LoadOptions struct {
	// FS는 파일 I/O 추상화 레이어다.
	// nil이면 os.DirFS("/")를 사용한다.
	// REQ-CFG-002: injectable fs.FS
	FS fs.FS
	// OverrideFiles는 기본 파일 조회 체인 대신 사용할 파일 경로 목록이다.
	// REQ-CFG-013: test-only override
	OverrideFiles []string
	// Logger는 로드 과정의 로그를 기록할 zap 로거다.
	// nil이면 zap.NewNop()을 사용한다.
	Logger *zap.Logger
	// GooseHome은 테스트에서 GOOSE_HOME을 강제 지정할 때 사용한다.
	// 빈 문자열이면 환경변수에서 읽는다.
	GooseHome string
	// WorkDir은 프로젝트 설정 파일 탐색 기준 디렉토리다.
	// 빈 문자열이면 os.Getwd()를 사용한다.
	WorkDir string
	// EnvOverrides는 테스트 격리용 환경변수 오버라이드 맵이다.
	// 설정된 경우 os.Getenv 대신 이 맵에서 값을 읽는다.
	// 실제 환경변수와 충돌 없이 병렬 테스트에서 사용 가능하다.
	EnvOverrides map[string]string
}

// Load는 계층형 설정을 로드하여 반환한다.
// 우선순위: defaults → project(YAML) → user(YAML) → runtime(env)
//
// @MX:ANCHOR: [AUTO] 모든 goosed 부트스트랩 + 후속 SPEC consumer가 호출하는 단일 진입점
// @MX:REASON: TRANSPORT/LLM/AGENT/CLI 등 모든 후속 SPEC이 이 함수로 시작함
// @MX:SPEC: SPEC-GOOSE-CONFIG-001 REQ-CFG-001
func Load(opts LoadOptions) (*Config, error) {
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	fsys := opts.FS
	if fsys == nil {
		fsys = os.DirFS("/")
	}

	// 1단계: 기본값으로 시작
	cfg := defaultConfig()
	sources := make(sourceMap)

	// 2단계: 파일 로딩 체인 결정
	if len(opts.OverrideFiles) > 0 {
		// REQ-CFG-013: OverrideFiles가 있으면 기본 파일 체인 bypass
		for _, path := range opts.OverrideFiles {
			if err := mergeYAMLFile(fsys, path, cfg, sources, SourceOverride, logger); err != nil {
				return nil, err
			}
		}
	} else {
		// 표준 파일 체인: user → project (낮음에서 높음 순)

		// REQ-CFG-011: GOOSE_HOME 미설정 시 $HOME/.goose fallback
		gooseHome := opts.GooseHome
		if gooseHome == "" {
			gooseHome = resolveGooseHome()
		}

		// user 설정 파일
		userCfgPath := filepath.Join(gooseHome, "config.yaml")
		if err := mergeYAMLFile(fsys, userCfgPath, cfg, sources, SourceUser, logger); err != nil {
			return nil, err
		}

		// project 설정 파일
		// REQ-CFG-005: cwd의 .goose/config.yaml
		workDir := opts.WorkDir
		if workDir == "" {
			var err error
			workDir, err = os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("작업 디렉토리 탐색 실패: %w", err)
			}
		}
		projectCfgPath := filepath.Join(workDir, ".goose", "config.yaml")
		if err := mergeYAMLFile(fsys, projectCfgPath, cfg, sources, SourceProject, logger); err != nil {
			return nil, err
		}
	}

	// envLookup은 환경변수 조회 함수다.
	// EnvOverrides가 설정된 경우 그 맵에서 읽고, 아니면 os.Getenv를 사용한다.
	envLookup := makeEnvLookup(opts.EnvOverrides)

	// 3단계: 환경변수 오버레이
	// REQ-CFG-006: env vars override all file-based sources
	if err := envOverlay(cfg, sources, logger, envLookup); err != nil {
		return nil, err
	}

	// 4단계: strict 모드 검사
	// REQ-CFG-014: GOOSE_CONFIG_STRICT=true이면 unknown key 발견 시 실패
	if strictVal := envLookup("GOOSE_CONFIG_STRICT"); strictVal == "true" {
		if len(cfg.Unknown) > 0 {
			keys := make([]string, 0, len(cfg.Unknown))
			for k := range cfg.Unknown {
				keys = append(keys, k)
			}
			return nil, &StrictUnknownError{Keys: keys}
		}
	}

	// 5단계: SkillsRoot 기본값 설정
	// yaml 또는 env에서 명시적으로 지정되지 않은 경우에만 적용.
	// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-002 step 7
	if cfg.SkillsRoot == "" {
		gh := opts.GooseHome
		if gh == "" {
			gh = resolveGooseHome()
		}
		cfg.SkillsRoot = filepath.Join(gh, "skills")
	}

	cfg.sources = sources
	return cfg, nil
}

// resolveGooseHome은 GOOSE_HOME 환경변수를 읽고, 비어있으면 $HOME/.goose를 반환한다.
// REQ-CFG-011: 단 1회만 $HOME을 참조한다.
func resolveGooseHome() string {
	if home := os.Getenv("GOOSE_HOME"); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".goose")
	}
	return filepath.Join(home, ".goose")
}

// makeEnvLookup은 EnvOverrides 맵을 우선 조회하는 envLookup 함수를 반환한다.
// 테스트 격리에 사용되며, nil이면 os.Getenv를 직접 호출한다.
func makeEnvLookup(overrides map[string]string) func(string) string {
	if overrides == nil {
		return os.Getenv
	}
	return func(key string) string {
		if v, ok := overrides[key]; ok {
			return v
		}
		return os.Getenv(key)
	}
}

// mergeYAMLFile은 fs.FS에서 YAML 파일을 읽어 cfg에 병합한다.
// 파일이 없으면 오류 없이 skip한다.
// REQ-CFG-009: YAML 구문 오류 시 *ConfigError 반환
func mergeYAMLFile(fsys fs.FS, path string, cfg *Config, sources sourceMap, src Source, logger *zap.Logger) error {
	// fs.FS에서는 절대 경로를 사용할 수 없으므로, os.DirFS("/")가 루트면 경로 처리 필요
	// path는 절대 경로일 수 있으므로 fs.FS로 열기 위해 변환한다
	fsPath := toFSPath(path)

	f, err := fsys.Open(fsPath)
	if err != nil {
		// 파일이 없으면 정상 (optional)
		if isNotExist(err) {
			return nil
		}
		return &ConfigError{
			File:       path,
			Msg:        fmt.Sprintf("파일 열기 실패: %s", err.Error()),
			Underlying: err,
		}
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return &ConfigError{
			File:       path,
			Msg:        fmt.Sprintf("파일 읽기 실패: %s", err.Error()),
			Underlying: err,
		}
	}

	return mergeYAMLData(data, path, cfg, sources, src, logger)
}

// mergeYAMLData는 YAML 바이트 데이터를 cfg에 presence-aware merge한다.
func mergeYAMLData(data []byte, path string, cfg *Config, sources sourceMap, src Source, logger *zap.Logger) error {
	// 1차 파싱: yaml.Node 트리로 수행하여 key presence 보존
	// REQ-CFG-015: presence-aware merge
	var rootNode yaml.Node
	if err := yaml.Unmarshal(data, &rootNode); err != nil {
		// yaml.v3 오류에서 라인/컬럼 추출 시도
		line, col := extractYAMLErrorPos(err)
		return &ConfigError{
			File:       path,
			Line:       line,
			Column:     col,
			Msg:        err.Error(),
			Underlying: ErrSyntax,
		}
	}

	// 빈 파일 또는 null document
	if rootNode.Kind == 0 {
		return nil
	}

	// 2차 파싱: 노드 트리에서 Config 구조체로 적용
	return applyNodeToConfig(&rootNode, cfg, sources, src, logger)
}

// Source는 지정된 dot-path 필드의 소스를 반환한다.
// REQ-CFG-001: Config.Source(path string) string
func (c *Config) Source(path string) Source {
	if c.sources == nil {
		return SourceDefault
	}
	return c.sources.get(path)
}

// Validate는 Config 필드의 유효성을 검사한다.
// REQ-CFG-007: Validate() 호출 후 IsValid() == true
func (c *Config) Validate() error {
	var errs []error

	// 포트 범위 검증: 1..65535
	// REQ-CFG-010: 포트가 설정된 경우 1..65535 범위여야 함
	// 0은 "미설정"이 아니라 명시적으로 잘못된 포트 값임
	if c.Transport.HealthPort != 0 && (c.Transport.HealthPort < 1 || c.Transport.HealthPort > 65535) {
		errs = append(errs, ErrInvalidField{
			Path: "transport.health_port",
			Msg:  "must be 1..65535",
		})
	}
	if c.Transport.GRPCPort < 1 || c.Transport.GRPCPort > 65535 {
		errs = append(errs, ErrInvalidField{
			Path: "transport.grpc_port",
			Msg:  "must be 1..65535",
		})
	}

	// log.level enum 검증
	if c.Log.Level != "" {
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[c.Log.Level] {
			errs = append(errs, ErrInvalidField{
				Path: "log.level",
				Msg:  fmt.Sprintf("must be one of debug|info|warn|error, got %q", c.Log.Level),
			})
		}
	}

	// ui.locale enum 검증
	if c.UI.Locale != "" {
		validLocales := map[string]bool{"en": true, "ko": true, "ja": true, "zh": true}
		if !validLocales[c.UI.Locale] {
			errs = append(errs, ErrInvalidField{
				Path: "ui.locale",
				Msg:  fmt.Sprintf("must be one of en|ko|ja|zh, got %q", c.UI.Locale),
			})
		}
	}

	// SPEC-GOOSE-CREDPOOL-001 OI-06: every CredentialConfig.Type must be
	// a known credential source kind. Unknown values are rejected up-front
	// so the credential factory never sees a bogus dispatch key.
	for providerName, provider := range c.LLM.Providers {
		for idx, cred := range provider.Credentials {
			if cred.Type == "" {
				errs = append(errs, ErrInvalidField{
					Path: fmt.Sprintf("llm.providers.%s.credentials[%d].type", providerName, idx),
					Msg:  "type must be a non-empty credential source kind",
				})
				continue
			}
			if !IsKnownCredentialType(cred.Type) {
				errs = append(errs, ErrInvalidField{
					Path: fmt.Sprintf("llm.providers.%s.credentials[%d].type", providerName, idx),
					Msg:  fmt.Sprintf("unknown credential source kind %q", cred.Type),
				})
			}
		}
	}

	if len(errs) > 0 {
		return errs[0] // 첫 번째 오류 반환 (향후 multi-error 확장 가능)
	}

	c.validated = true
	return nil
}

// IsValid는 Validate()가 성공적으로 호출되었는지 여부를 반환한다.
// REQ-CFG-007: Validate() 호출 전 false, 성공 후 true
func (c *Config) IsValid() bool {
	return c.validated
}

// Redacted는 secret-typed 필드를 마스킹한 인간 가독 스냅샷을 반환한다.
// REQ-CFG-017: Config.Redacted() — secret masking
//
// @MX:NOTE: [AUTO] secret 마스킹 — 로그 출력 시 이 메서드를 사용할 것
// @MX:SPEC: REQ-CFG-017
func (c *Config) Redacted() string {
	// Config의 복사본을 만들어 secret 필드를 마스킹
	redacted := *c
	if redacted.LLM.Providers != nil {
		providers := make(map[string]ProviderConfig, len(redacted.LLM.Providers))
		for name, p := range redacted.LLM.Providers {
			if p.APIKey != "" {
				p.APIKey = maskSecret()
			}
			providers[name] = p
		}
		redacted.LLM.Providers = providers
	}

	// YAML 형식으로 직렬화
	data, err := yaml.Marshal(redacted)
	if err != nil {
		return fmt.Sprintf("config redact error: %v", err)
	}
	return string(data)
}

// maskSecret은 8자 고정 길이 마스크를 반환한다.
// REQ-CFG-017: constant-length mask "sk-*****" (sk- prefix + 5 asterisks = 8 chars)
func maskSecret() string {
	return "sk-*****"
}

// LoadFromMap은 테스트용 in-memory 로더다. map에서 Config를 생성한다.
// REQ-CFG-013 §3.1 7번: dry-run API
func LoadFromMap(m map[string]any) (*Config, error) {
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("map 직렬화 실패: %w", err)
	}

	cfg := defaultConfig()
	sources := make(sourceMap)
	logger := zap.NewNop()

	if err := mergeYAMLData(data, "<map>", cfg, sources, SourceUser, logger); err != nil {
		return nil, err
	}

	if err := envOverlay(cfg, sources, logger, os.Getenv); err != nil {
		return nil, err
	}

	cfg.sources = sources
	return cfg, nil
}

// toFSPath는 절대 경로를 fs.FS에서 사용 가능한 상대 경로로 변환한다.
// os.DirFS("/")는 "/"로 시작하지 않는 경로를 사용한다.
func toFSPath(path string) string {
	if len(path) > 0 && path[0] == '/' {
		return path[1:] // 선행 "/" 제거
	}
	return path
}

// isNotExist는 파일 미존재 오류를 확인한다.
func isNotExist(err error) bool {
	return os.IsNotExist(err)
}

// extractYAMLErrorPos는 yaml.v3 오류에서 라인/컬럼 정보를 추출한다.
func extractYAMLErrorPos(err error) (line, col int) {
	var yamlErr *yaml.TypeError
	if ok := isYAMLTypeError(err, &yamlErr); ok {
		// yaml.TypeError는 라인 정보를 포함하지 않음
		return 0, 0
	}
	// yaml.v3 파서 오류에서 라인 추출 시도
	// yaml.v3은 "yaml: line N: ..." 형식 문자열을 사용함
	msg := err.Error()
	var l int
	if n, _ := fmt.Sscanf(msg, "yaml: line %d:", &l); n == 1 {
		return l, 0
	}
	return 0, 0
}

// isYAMLTypeError는 에러가 *yaml.TypeError인지 확인한다.
func isYAMLTypeError(err error, target **yaml.TypeError) bool {
	te, ok := err.(*yaml.TypeError)
	if ok && target != nil {
		*target = te
	}
	return ok
}

// applyNodeToConfig는 yaml.Node 트리를 Config 구조체에 presence-aware하게 적용한다.
// REQ-CFG-015: presence-aware merge — "absent"와 "zero-value"를 구분
func applyNodeToConfig(rootNode *yaml.Node, cfg *Config, sources sourceMap, src Source, logger *zap.Logger) error {
	// document 노드이면 content[0]이 실제 맵 노드
	node := rootNode
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}

	if node.Kind != yaml.MappingNode {
		return nil
	}

	// 최상위 키를 순회하며 known/unknown 분리
	knownKeys := map[string]bool{
		"log": true, "transport": true, "llm": true, "learning": true, "ui": true, "audit": true,
	}

	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		key := keyNode.Value

		if !knownKeys[key] {
			// REQ-CFG-008: unknown top-level keys 보존
			if cfg.Unknown == nil {
				cfg.Unknown = make(map[string]any)
			}
			var val any
			if err := valNode.Decode(&val); err == nil {
				cfg.Unknown[key] = val
			}
			logger.Warn("알 수 없는 설정 키", zap.String("key", key))
			continue
		}

		// known 키 처리
		switch key {
		case "log":
			if err := applyLogNode(valNode, cfg, sources, src); err != nil {
				return err
			}
		case "transport":
			if err := applyTransportNode(valNode, cfg, sources, src); err != nil {
				return err
			}
		case "llm":
			if err := applyLLMNode(valNode, cfg, sources, src); err != nil {
				return err
			}
		case "learning":
			if err := applyLearningNode(valNode, cfg, sources, src); err != nil {
				return err
			}
		case "ui":
			if err := applyUINode(valNode, cfg, sources, src); err != nil {
				return err
			}
		case "audit":
			if err := applyAuditNode(valNode, cfg, sources, src); err != nil {
				return err
			}
		}
	}

	return nil
}

// applyLogNode는 "log" 섹션 노드를 Config.Log에 적용한다.
func applyLogNode(node *yaml.Node, cfg *Config, sources sourceMap, src Source) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		switch k {
		case "level":
			// REQ-CFG-012: 쉘 변수 확장 금지 — yaml이 파싱한 값을 그대로 사용
			cfg.Log.Level = v.Value
			sources.set("log.level", src)
		}
	}
	return nil
}

// applyTransportNode는 "transport" 섹션 노드를 Config.Transport에 적용한다.
func applyTransportNode(node *yaml.Node, cfg *Config, sources sourceMap, src Source) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		switch k {
		case "health_port":
			port, err := strconv.Atoi(v.Value)
			if err != nil {
				return ErrInvalidField{
					Path:     "transport.health_port",
					Expected: "int",
					Got:      "string",
					Msg:      fmt.Sprintf("정수 파싱 실패: %s", v.Value),
				}
			}
			cfg.Transport.HealthPort = port
			sources.set("transport.health_port", src)
		case "grpc_port":
			port, err := strconv.Atoi(v.Value)
			if err != nil {
				return ErrInvalidField{
					Path:     "transport.grpc_port",
					Expected: "int",
					Got:      "string",
					Msg:      fmt.Sprintf("정수 파싱 실패: %s", v.Value),
				}
			}
			cfg.Transport.GRPCPort = port
			sources.set("transport.grpc_port", src)
		}
	}
	return nil
}

// applyLLMNode는 "llm" 섹션 노드를 Config.LLM에 적용한다.
func applyLLMNode(node *yaml.Node, cfg *Config, sources sourceMap, src Source) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		switch k {
		case "default_provider":
			cfg.LLM.DefaultProvider = v.Value
			sources.set("llm.default_provider", src)
		case "providers":
			if err := applyProvidersNode(v, cfg, sources, src); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyProvidersNode는 "providers" 맵 노드를 Config.LLM.Providers에 적용한다.
func applyProvidersNode(node *yaml.Node, cfg *Config, sources sourceMap, src Source) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	if cfg.LLM.Providers == nil {
		cfg.LLM.Providers = make(map[string]ProviderConfig)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		providerName := node.Content[i].Value
		providerNode := node.Content[i+1]
		existing := cfg.LLM.Providers[providerName]
		if err := applyProviderNode(providerNode, &existing, sources, src, providerName); err != nil {
			return err
		}
		cfg.LLM.Providers[providerName] = existing
	}
	return nil
}

// applyProviderNode는 개별 provider 노드를 ProviderConfig에 적용한다.
func applyProviderNode(node *yaml.Node, p *ProviderConfig, sources sourceMap, src Source, name string) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		switch k {
		case "host":
			p.Host = v.Value
			sources.set(fmt.Sprintf("llm.providers.%s.host", name), src)
		case "api_key":
			p.APIKey = v.Value
			sources.set(fmt.Sprintf("llm.providers.%s.api_key", name), src)
		case "credentials":
			// SPEC-GOOSE-CREDPOOL-001 OI-06: parse the credentials sequence.
			if err := applyCredentialsNode(v, p, sources, src, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// applyCredentialsNode parses a "credentials" sequence node into
// ProviderConfig.Credentials. Order is preserved as the YAML document
// declares it. Unknown keys inside an entry are silently ignored.
// SPEC-GOOSE-CREDPOOL-001 OI-06.
func applyCredentialsNode(node *yaml.Node, p *ProviderConfig, sources sourceMap, src Source, providerName string) error {
	if node.Kind != yaml.SequenceNode {
		return nil
	}
	entries := make([]CredentialConfig, 0, len(node.Content))
	for _, entryNode := range node.Content {
		if entryNode.Kind != yaml.MappingNode {
			continue
		}
		var entry CredentialConfig
		for i := 0; i+1 < len(entryNode.Content); i += 2 {
			k := entryNode.Content[i].Value
			v := entryNode.Content[i+1]
			switch k {
			case "type":
				entry.Type = v.Value
			case "path":
				entry.Path = v.Value
			case "keyring_ref":
				entry.KeyringRef = v.Value
			}
		}
		entries = append(entries, entry)
	}
	p.Credentials = entries
	sources.set(fmt.Sprintf("llm.providers.%s.credentials", providerName), src)
	return nil
}

// applyLearningNode는 "learning" 섹션 노드를 Config.Learning에 적용한다.
func applyLearningNode(node *yaml.Node, cfg *Config, sources sourceMap, src Source) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		switch k {
		case "enabled":
			// REQ-CFG-015: bool zero-value (false)도 presence-aware로 처리
			b, err := strconv.ParseBool(v.Value)
			if err != nil {
				return ErrInvalidField{
					Path:     "learning.enabled",
					Expected: "bool",
					Got:      "string",
					Msg:      fmt.Sprintf("bool 파싱 실패: %s", v.Value),
				}
			}
			cfg.Learning.Enabled = b
			sources.set("learning.enabled", src)
		}
	}
	return nil
}

// applyUINode는 "ui" 섹션 노드를 Config.UI에 적용한다.
func applyUINode(node *yaml.Node, cfg *Config, sources sourceMap, src Source) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		switch k {
		case "locale":
			cfg.UI.Locale = v.Value
			sources.set("ui.locale", src)
		}
	}
	return nil
}

// applyAuditNode는 "audit" 섹션 노드를 Config.Audit에 적용한다.
func applyAuditNode(node *yaml.Node, cfg *Config, sources sourceMap, src Source) error {
	if node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := node.Content[i].Value
		v := node.Content[i+1]
		switch k {
		case "enabled":
			// REQ-CFG-015: bool zero-value (false)도 presence-aware로 처리
			b, err := strconv.ParseBool(v.Value)
			if err != nil {
				return ErrInvalidField{
					Path:     "audit.enabled",
					Expected: "bool",
					Got:      "string",
					Msg:      fmt.Sprintf("bool 파싱 실패: %s", v.Value),
				}
			}
			cfg.Audit.Enabled = b
			sources.set("audit.enabled", src)
		case "max_size_mb":
			size, err := strconv.Atoi(v.Value)
			if err != nil {
				return ErrInvalidField{
					Path:     "audit.max_size_mb",
					Expected: "int",
					Got:      "string",
					Msg:      fmt.Sprintf("int 파싱 실패: %s", v.Value),
				}
			}
			cfg.Audit.MaxSizeMB = size
			sources.set("audit.max_size_mb", src)
		case "global_dir":
			cfg.Audit.GlobalDir = v.Value
			sources.set("audit.global_dir", src)
		case "local_dir":
			cfg.Audit.LocalDir = v.Value
			sources.set("audit.local_dir", src)
		}
	}
	return nil
}

// FSAccessConfig is the filesystem access control configuration.
// SPEC-GOOSE-FS-ACCESS-001
type FSAccessConfig struct {
	// Enabled controls whether FS access control is active. Default: true.
	Enabled bool `yaml:"enabled"`
	// PolicyPath is the path to the security.yaml policy file.
	// Default: ./.goose/config/security.yaml
	PolicyPath string `yaml:"policy_path"`
	// ReloadInterval controls how often the policy file is checked for changes.
	// Default: 5s.
	ReloadInterval string `yaml:"reload_interval"`
}
