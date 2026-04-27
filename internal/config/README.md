# config

Goose 데몬의 계층형 설정 로더를 제공합니다. 다중 우선순위 설정 소스를 통합하여 불변 설정 객체를 생성합니다.

## 개요

`config` 패키지는 YAML 파일, 환경변수, 기본값을 계층적으로 로드하여 통합 설정을 제공합니다. 모든 설정은 불변(immutable)이며 로딩 후 수정이 금지됩니다.

## 로딩 우선순위

설정 로딩은 다음 우선순위로 병합됩니다 (낮음 → 높음):

```
defaults → project(YAML) → user(YAML) → runtime(env)
```

1. **defaults**: 하드코딩된 기본값
2. **project**: `.moai/config/config.yaml` (프로젝트 설정)
3. **user**: `~/.config/goose/config.yaml` (사용자 설정)
4. **runtime**: 환경변수 (런타임 오버라이드)

## 주요 컴포넌트

### Config

전체 설정 구조체입니다. 불변이므로 로딩 후 필드 변경이 금지됩니다.

```go
type Config struct {
    Log       LogConfig       `yaml:"log"`
    Transport TransportConfig `yaml:"transport"`
    LLM       LLMConfig       `yaml:"llm"`
    // ... 기타 설정
}
```

### Load 함수

```go
func Load() (*Config, error)
```

모든 우선순위를 순회하며 설정을 로드하고 병합합니다.

**REQ-CFG-003**: concurrent reads safe (effectively frozen pointer)

## 설정 구조

### LogConfig

```go
type LogConfig struct {
    Level  string // "debug" | "info" | "warn" | "error"
    Format string // "json" | "console"
    Output string // 출력 경로 (파일 또는 stdout/stderr)
}
```

### TransportConfig

```go
type TransportConfig struct {
    Type     string // "grpc" | "http"
    Address  string // 서버 주소
    TLS      TLSConfig
}
```

### LLMConfig

```go
type LLMConfig struct {
    Provider     string            // "anthropic" | "openai" | "custom"
    ModelID      string            // 모델 식별자
    APIKey       string            // API 키 (환경변수 우선)
    MaxTokens    int               // 기본 max_tokens
    Temperature  float64           // 기본 temperature
    ExtraParams  map[string]any    // 추가 파라미터
}
```

## 사용 예시

### 기본 로딩

```go
import "github.com/modu-ai/goose/internal/config"

cfg, err := config.Load()
if err != nil {
    log.Fatal("config load failed", err)
}

// 설정 사용 (읽기 전용)
logger := initLogger(cfg.Log)
```

### 환경변수 오버라이드

```bash
# LLM provider 오버라이드
export GOOSE_LLM_PROVIDER=anthropic
export GOSE_LLM_MODEL_ID=claude-3-5-sonnet-20241022
export GOSE_LLM_API_KEY=sk-ant-...

# 로그 레벨 오버라이드
export GOOSE_LOG_LEVEL=debug
```

### YAML 설정 파일

**프로젝트 설정** (`.moai/config/config.yaml`):

```yaml
log:
  level: info
  format: json
  output: stdout

llm:
  provider: anthropic
  model_id: claude-3-5-haiku-20241022
  max_tokens: 4096
```

**사용자 설정** (`~/.config/goose/config.yaml`):

```yaml
log:
  level: debug  # 프로젝트 설정 오버라이드
```

## 설정 검증

`Load()` 함수는 설정의 유효성을 검증합니다:

```go
type ValidationError struct {
    Field   string // 문제 필드 경로 (예: "llm.model_id")
    Message string // 오류 메시지
}
```

**검증 항목**:
- 필수 필드 존재 여부
- 열거형 값 범위 (예: log.level은 debug/info/warn/error 중 하나)
- 경로 유효성 (파일 존재 여부)
- API 키 형식

## 크로스 패키지 인터페이스

### @MX:ANCHOR

`Load()`는 모든 후속 SPEC의 단일 진입점이므로 fan_in >= 5입니다:

```go
// @MX:ANCHOR: [AUTO] 모든 goosed 부트스트랩 + 후속 SPEC consumer가 호출하는 단일 진입점
// @MX:REASON: Load()는 TRANSPORT/LLM/AGENT/CLI 등 모든 후속 SPEC의 시작점이므로 fan_in >= 5 예상
func Load() (*Config, error)
```

## SPEC 참조

본 패키지는 **SPEC-GOOSE-CONFIG-001**에 의해 정의됩니다.

- REQ-CFG-001 ~ REQ-CFG-015: 모든 요구사항 충족
- 테스트 커버리지: 85.8%
- 불변성 보장 (REQ-CFG-003)

## 의존성

- `io/fs`: 파일 시스템 순회
- `os`: 환경변수 읽기
- `path/filepath`: 경로 조작
- `gopkg.in/yaml.v3`: YAML 파싱
- `go.uber.org/zap`: 로깅

## 테스트

```bash
go test ./internal/config/...
go test -race ./internal/config/...
go test -cover ./internal/config/...
```

## 보안 고려사항

### API 키 관리

API 키는 절대 YAML 파일에 포함하지 마십시오. 환경변수로만 전달하세요:

```yaml
# ❌ 잘못된 예 (YAML에 API 키 포함)
llm:
  api_key: sk-ant-xxx...

# ✅ 올바른 예 (환경변수 사용)
llm:
  api_key: ${GOOSE_LLM_API_KEY}  # 런타임에 환경변수 주입
```

### 설정 파일 권한

설정 파일은 사용자 전용(0600)으로 유지하세요:

```bash
chmod 600 ~/.config/goose/config.yaml
```

## 에러 처리

### ConfigNotFoundError

설정 파일을 찾을 수 없을 때:

```go
if errors.Is(err, config.ErrConfigNotFound) {
    // 기본값 사용 계속 진행
    log.Warn("config file not found, using defaults")
}
```

### ValidationError

설정 값이 유효하지 않을 때:

```go
var ve config.ValidationError
if errors.As(err, &ve) {
    log.Error("invalid config",
        zap.String("field", ve.Field),
        zap.String("message", ve.Message),
    )
}
```
