// Package aliasconfig는 모델 별칭(alias) 설정 파일 로더를 제공한다.
// SPEC-GOOSE-ALIAS-CONFIG-001
package aliasconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"slices"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/modu-ai/goose/internal/command"
	"github.com/modu-ai/goose/internal/llm/router"
	"go.uber.org/zap"
)

// Sentinel errors
var (
	// ErrConfigNotFound는 aliases.yaml 파일을 찾을 수 없을 때 반환된다.
	ErrConfigNotFound = errors.New("aliasconfig: config file not found")
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
	fsys       fs.FS
	logger     Logger
	stdLogger  *log.Logger
}

// Options는 Loader 생성 옵션이다.
type Options struct {
	// ConfigPath는 설정 파일 경로이다. 비어 있으면 기본 경로 사용.
	ConfigPath string
	// FS는 파일 시스템 인터페이스이다. 비어 있으면 os.DirFS("/") 사용.
	FS fs.FS
	// Logger는 구조화된 로거이다. 비어 있으면 zap.NewNop() 사용.
	Logger Logger
	// StdLogger는 표준 로거이다. 비어 있으면 logging 비활성화.
	StdLogger *log.Logger
}

// New는 새 Loader를 생성한다.
func New(opts Options) *Loader {
	configPath := opts.ConfigPath
	if configPath == "" {
		// P3: Check project-local overlay first
		if projectLocalPath := detectProjectLocalAliasFile(); projectLocalPath != "" {
			configPath = projectLocalPath
		} else if home := os.Getenv(homeEnv); home != "" {
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

	filesystem := opts.FS
	if filesystem == nil {
		// When no custom filesystem provided, use os.ReadFile directly for full path support
		filesystem = &osFS{}
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Loader{
		configPath: configPath,
		fsys:       filesystem,
		logger:     logger,
		stdLogger:  opts.StdLogger,
	}
}

// AliasConfig는 alias 설정 파일 구조이다.
type AliasConfig struct {
	// Aliases는 별칭 → provider/model 맵핑이다.
	Aliases map[string]string `yaml:"aliases"`
}

// maxAliasFileSize is the maximum allowed size for the alias file (1 MiB).
const maxAliasFileSize = 1 * 1024 * 1024

// detectProjectLocalAliasFile는 프로젝트 로컬 별칭 파일을 감지한다.
// $CWD/.goose/aliases.yaml 경로를 확인하고 파일이 존재하면 경로를 반환한다.
// P3 기능: 프로젝트 로컬 설정 오버레이 지원.
func detectProjectLocalAliasFile() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	projectLocalPath := filepath.Join(cwd, ".goose", "aliases.yaml")
	if _, err := os.Stat(projectLocalPath); err == nil {
		return projectLocalPath
	}

	return ""
}

// Load는 설정 파일을 로드하고 alias 맵을 반환한다.
// P3 기능: fs.FS 인터페이스를 사용하여 테스트 가능한 파일 읽기.
func (l *Loader) Load() (map[string]string, error) {
	l.logger.Debug("loading alias config", zap.String("path", l.configPath))

	if l.stdLogger != nil {
		l.stdLogger.Printf("[aliasconfig] loading config from: %s", l.configPath)
	}

	// Check file size before reading to prevent large file parsing
	fileInfo, err := fs.Stat(l.fsys, l.configPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			l.logger.Info("alias config not found, using empty map", zap.String("path", l.configPath))
			if l.stdLogger != nil {
				l.stdLogger.Printf("[aliasconfig] config file not found: %s", l.configPath)
			}
			return nil, nil // File not found is not an error (return nil map)
		}
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}

	// Enforce 1 MiB size limit
	if fileInfo.Size() > maxAliasFileSize {
		if l.stdLogger != nil {
			l.stdLogger.Printf("[aliasconfig] file too large: %d bytes (max %d)", fileInfo.Size(), maxAliasFileSize)
		}
		return nil, command.ErrAliasFileTooLarge
	}

	// Read file content using fs.FS interface
	data, err := fs.ReadFile(l.fsys, l.configPath)
	if err != nil {
		// Wrap permission errors with file path context for better debugging
		if errors.Is(err, fs.ErrPermission) {
			if l.stdLogger != nil {
				l.stdLogger.Printf("[aliasconfig] permission denied reading: %s", l.configPath)
			}
			return nil, fmt.Errorf("aliasconfig: permission denied reading %s: %w", l.configPath, err)
		}
		return nil, fmt.Errorf("%w: %w", ErrConfigNotFound, err)
	}

	// Parse YAML
	var config AliasConfig
	if err := yamlUnmarshal(data, &config); err != nil {
		if l.stdLogger != nil {
			l.stdLogger.Printf("[aliasconfig] yaml parsing error: %v", err)
		}
		return nil, fmt.Errorf("%w: %w", command.ErrMalformedAliasFile, err)
	}

	l.logger.Info("alias config loaded", zap.Int("aliases", len(config.Aliases)))
	if l.stdLogger != nil {
		l.stdLogger.Printf("[aliasconfig] loaded %d alias entries", len(config.Aliases))
	}

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

	for alias, canonical := range aliasMap {
		// Check for empty alias key
		if alias == "" {
			errs = append(errs, command.ErrEmptyAliasEntry)
			continue
		}

		// Check for empty canonical value
		if canonical == "" {
			errs = append(errs, command.ErrEmptyAliasEntry)
			continue
		}

		// Validate canonical format (provider/model)
		provider, model, ok := parseModelTarget(canonical)
		if !ok {
			errs = append(errs, command.ErrInvalidCanonical)
			continue
		}

		// Strict mode: validate provider and model
		if strict && registry != nil {
			// Check if provider exists in registry
			if _, exists := registry.Get(provider); !exists {
				errs = append(errs, fmt.Errorf("%w: %s", command.ErrUnknownProviderInAlias, provider))
				continue
			}

			// Check if model exists in provider's suggested models
			meta, _ := registry.Get(provider)
			if !slices.Contains(meta.SuggestedModels, model) {
				errs = append(errs, fmt.Errorf("%w: %s", command.ErrUnknownModelInAlias, model))
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

// osFS는 os 패키지 함수를 사용하는 fs.FS 구현체이다.
// P3 기능: 기본 파일 시스템으로 os.DirFS 대신 절대 경로 지원.
type osFS struct{}

// Open는 fs.FS 인터페이스를 구현한다.
func (o *osFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}
