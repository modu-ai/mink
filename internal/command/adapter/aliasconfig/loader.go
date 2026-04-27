// Package aliasconfig는 모델 별칭(alias) 설정 파일 로더를 제공한다.
// SPEC-GOOSE-ALIAS-CONFIG-001
package aliasconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/modu-ai/goose/internal/llm/router"
	"go.uber.org/zap"
)

// Sentinel errors
var (
	// ErrConfigNotFound는 aliases.yaml 파일을 찾을 수 없을 때 반환된다.
	ErrConfigNotFound = errors.New("aliasconfig: config file not found")

	// ErrInvalidFormat은 YAML 파싱 실패 시 반환된다.
	ErrInvalidFormat = errors.New("aliasconfig: invalid YAML format")

	// ErrInvalidAlias는 별칭 형식이 올바르지 않을 때 반환된다.
	ErrInvalidAlias = errors.New("aliasconfig: invalid alias format")

	// ErrInvalidTarget은 대상(provider/model) 형식이 올바르지 않을 때 반환된다.
	ErrInvalidTarget = errors.New("aliasconfig: invalid target format")

	// ErrUnknownProvider는 대상의 provider를 찾을 수 없을 때 반환된다.
	ErrUnknownProvider = errors.New("aliasconfig: unknown provider")

	// ErrValidationFailed는 검증 실패 시 반환된다.
	ErrValidationFailed = errors.New("aliasconfig: validation failed")
)

// defaultConfigPath는 기본 설정 파일 경로이다.
const defaultConfigPath = ".goose/aliases.yaml"

// homeEnv는 홈 디렉토리 환경변수 키이다.
const homeEnv = "GOOSE_HOME"

// Logger는 로깅 인터페이스이다.
type Logger interface {
	Debug(string, ...zap.Field)
	Info(string, ...zap.Field)
	Warn(string, ...zap.Field)
	Error(string, ...zap.Field)
}

// Loader는 alias 설정 로더이다.
type Loader struct {
	configPath string
	logger     Logger
}

// Options는 Loader 생성 옵션이다.
type Options struct {
	// ConfigPath는 설정 파일 경로이다. 비어 있으면 기본 경로 사용.
	ConfigPath string
	// Logger는 로거이다. 비어 있으면 zap.NewNop() 사용.
	Logger Logger
}

// New는 새 Loader를 생성한다.
func New(opts Options) *Loader {
	configPath := opts.ConfigPath
	if configPath == "" {
		if home := os.Getenv(homeEnv); home != "" {
			configPath = filepath.Join(home, "aliases.yaml")
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				configPath = filepath.Join(homeDir, defaultConfigPath)
			} else {
				configPath = filepath.Join(homeDir, defaultConfigPath)
			}
		}
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Loader{
		configPath: configPath,
		logger:     logger,
	}
}

// AliasConfig는 alias 설정 파일 구조이다.
type AliasConfig struct {
	// Aliases는 별칭 → provider/model 맵핑이다.
	Aliases map[string]string `yaml:"aliases"`
}

// Load는 설정 파일을 로드하고 alias 맵을 반환한다.
func (l *Loader) Load() (map[string]string, error) {
	l.logger.Debug("loading alias config", zap.String("path", l.configPath))

	// 파일 존재 확인은 선택사항 (없으면 빈 맵 반환)
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			l.logger.Info("alias config not found, using empty map", zap.String("path", l.configPath))
			return nil, nil // 파일 없으면 nil 반환 (정상)
		}
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}

	// YAML 파싱
	var config AliasConfig
	if err := yamlUnmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidFormat, err)
	}

	l.logger.Info("alias config loaded", zap.Int("aliases", len(config.Aliases)))
	return config.Aliases, nil
}

// LoadDefault는 기본 경로에서 설정을 로드한다.
func (l *Loader) LoadDefault() (map[string]string, error) {
	return l.Load()
}

// Validate는 alias 맵을 검증하고 에러 목록을 반환한다.
// strict=true인 경우 에러가 있으면 빈 맵을 반환하지 않는다.
func Validate(aliasMap map[string]string, registry *router.ProviderRegistry, strict bool) []error {
	if aliasMap == nil {
		return nil
	}

	var errs []error

	for alias, target := range aliasMap {
		// 별칭 형식 검증 (비어 있지 않음)
		if alias == "" {
			errs = append(errs, fmt.Errorf("%w: empty alias", ErrInvalidAlias))
			continue
		}

		// 대상 형식 검증 (provider/model 형식)
		provider, _, ok := parseModelTarget(target)
		if !ok {
			errs = append(errs, fmt.Errorf("%w: alias=%s target=%s", ErrInvalidTarget, alias, target))
			continue
		}

		// provider 존재 검증
		if registry != nil {
			if _, exists := registry.Get(provider); !exists {
				errs = append(errs, fmt.Errorf("%w: alias=%s provider=%s", ErrUnknownProvider, alias, provider))
			}
		}
	}

	return errs
}

// parseModelTarget은 "provider/model" 형식의 문자열을 파싱한다.
func parseModelTarget(target string) (provider, model string, ok bool) {
	// 슬래시가 하나여야 함 (provider/model)
	slashCount := 0
	slashIndex := -1
	for i := 0; i < len(target); i++ {
		if target[i] == '/' {
			slashCount++
			if slashCount == 1 {
				slashIndex = i
			}
		}
	}

	// 슬래시가 정확히 하나가 아니면 invalid
	if slashCount != 1 {
		return "", "", false
	}

	provider = target[:slashIndex]
	model = target[slashIndex+1:]

	// provider나 model이 비어 있으면 invalid
	if provider == "" || model == "" {
		return "", "", false
	}

	return provider, model, true
}

// yamlUnmarshal는 YAML 언마샬링 함수이다.
func yamlUnmarshal(data []byte, config *AliasConfig) error {
	return yaml.Unmarshal(data, config)
}
