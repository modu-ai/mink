# internal/plugin

**Plugin System 패키지** — 플러그인 로딩, 검증, 생애주기 관리

## 개요

본 패키지는 MINK의 **Plugin 시스템**을 구현합니다. 외부 플러그인을 안전하게 로드하고, manifest 검증, 의존성 해석, 생애주기 관리(init → activate → deactivate → unload)를 수행합니다.

## 핵심 기능

### Plugin Manifest

YAML/JSON 기반 플러그인 정의:

```go
type Manifest struct {
    ID          string            `yaml:"id"`
    Name        string            `yaml:"name"`
    Version     string            `yaml:"version"`
    Description string            `yaml:"description"`
    Skills      []string          `yaml:"skills"`
    Tools       []string          `yaml:"tools"`
    Agents      []string          `yaml:"agents"`
    Permissions []string          `yaml:"permissions"`
    Config      map[string]any    `yaml:"config"`
}
```

### Plugin Lifecycle

```
Load → Validate → Init → Activate → (Running) → Deactivate → Unload
```

### Plugin Registry

```go
type Registry struct {
    mu      sync.RWMutex
    plugins map[string]*Plugin
}

func (r *Registry) Load(path string) error
func (r *Registry) Activate(id string) error
func (r *Registry) Deactivate(id string) error
func (r *Registry) Get(id string) (*Plugin, bool)
```

## 파일 구조

```
internal/plugin/
├── plugin.go       # Plugin 구조체 및 Lifecycle
├── manifest.go     # Manifest 파싱 및 검증
├── registry.go     # Plugin 레지스트리
└── *_test.go       # 테스트
```

## 관련 SPEC

- **SPEC-GOOSE-PLUGIN-001**: 본 패키지의 주요 SPEC
- **SPEC-GOOSE-SKILLS-001**: Plugin manifest의 skills 배열 로드

---

Version: 1.0.0
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-PLUGIN-001
