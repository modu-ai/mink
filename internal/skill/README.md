# internal/skill

**Progressive Disclosure Skill System 패키지** — Claude Code 호환 YAML frontmatter 기반 Skill 로딩 및 실행

## 개요

본 패키지는 MINK의 **Skill 시스템**을 구현합니다. Claude Code의 Progressive Disclosure(L0~L3 effort 티어) + YAML frontmatter + 4-trigger 활성화 모델을 Go로 포팅하여, `QueryEngine`(`QUERY-001`)이 매 iteration 직전 적절한 Skill을 로드·치환·권한 검증한 후 system/user prompt에 반영합니다.

## 핵심 기능

### Skill 로딩

디스크 상의 `SKILL.md` 파일을 walk하여 `SkillDefinition` 레지스트리를 구성:

```go
registry := skill.NewRegistry()
definitions, err := loader.LoadSkillsDir(skillRoot, registry)
if err != nil {
    return err
}

for _, def := range definitions {
    log.Printf("Loaded skill: %s (L%d, %d tokens)", def.ID, def.Effort, def.EstimatedTokens)
}
```

### YAML Frontmatter 파싱

**Allowlist 기반 파싱** (알 수 없는 속성은 default-deny):

```go
const SAFE_SKILL_PROPERTIES = "allowed_tools|allowed_agents|description|effort|disable|force"

func ParseFrontmatter(data []byte) (*Frontmatter, error) {
    var fm Frontmatter
    if err := yaml.Unmarshal(data, &fm); err != nil {
        return nil, err
    }

    // validate against allowlist
    if err := validateFrontmatter(&fm); err != nil {
        return nil, err
    }

    return &fm, nil
}
```

### 4-Trigger 활성화 모델

각 Skill은 4가지 활성화 조건 중 하나 이상을 만족해야 로드됩니다:

| Trigger | 설명 | 예시 |
|---------|------|------|
| **inline** | 명시적 포함 (paths 매칭) | `paths: "**/*.go"` |
| **fork** | 현재 파일에서 fork | `fork: true` |
| **conditional** | 조건부 로직 | `if: language == "go"` |
| **remote** | 원격 URL | `url: https://...` |

### Progressive Disclosure

**토큰 비용 최소화**를 위해 frontmatter만 먼저 파싱:

```go
func estimateSkillFrontmatterTokens(frontmatter string) int {
    // L0=50, L1=200, L2=500, L3=1000+
    // frontmatter만 먼저 파싱하여 발견 비용 최소화
}
```

| Level | Token Budget | 설명 |
|-------|--------------|------|
| L0 | ~50 tokens | 메타데이터만 |
| L1 | ~200 tokens | 요약 + 핵심 기능 |
| L2 | ~500 tokens | 상세 기능 설명 |
| L3 | ~1000+ tokens | 전체 본문 |

## 핵심 구성 요소

### SkillDefinition

```go
type SkillDefinition struct {
    ID          string           // skill 식별자
    Name        string           // 표시 이름
    Description string           // 설명
    Effort      EffortLevel      // L0~L3
    Body        string           // skill 본문 (markdown)
    Frontmatter Frontmatter      // YAML frontmatter
    Enabled     bool             // 활성화 여부
    Trigger     TriggerType      // inline/fork/conditional/remote
    Paths       []string         // paths: 패턴 (gitignore 문법)
}
```

### SkillRegistry

```go
type SkillRegistry struct {
    mu        sync.RWMutex
    skills    map[string]*SkillDefinition  // ID -> Definition
    fileWatcher *FileWatcher               // 변경 감지
}

func (r *SkillRegistry) Load(definition *SkillDefinition) error
func (r *SkillRegistry) Unload(id string) error
func (r *SkillRegistry) Get(id string) (*SkillDefinition, bool)
func (r *SkillRegistry) MatchedFiles(paths []string) []*SkillDefinition
```

### ConditionalLoader

조건부 로직 평가:

```go
type ConditionalLoader struct {
    env map[string]string
}

func (l *ConditionalLoader) Evaluate(condition string) (bool, error) {
    // if: language == "go" && framework == "echo"
    // 환경 변수, 파일 존재, git 상태 등 평가
}
```

## 변수 치환

skill 본문 로드 시점에 특수 변수 치환:

| 변수 | 설명 | 예시 |
|------|------|------|
| `${CLAUDE_SKILL_DIR}` | skill 루트 디렉토리 | `~/.goose/skills/` |
| `${CLAUDE_SESSION_ID}` | 현재 세션 ID | `uuid-string` |

```go
func expandVariables(body string, vars map[string]string) string {
    result := body
    for key, value := range vars {
        result = strings.ReplaceAll(result, "${"+key+"}", value)
    }
    return result
}
```

## Paths 조건부 매칭

**gitignore 문법** (`!` 부정 포함):

```go
import "github.com/denormal/go-gitignore"

func matchPath(patterns []string, path string) bool {
    matcher := gitignore.NewMatcher(patterns)
    return matcher.Match(path, false)
}
```

예시:
```yaml
paths:
  - "**/*.go"           # 모든 .go 파일
  - "!**/*_test.go"     # 테스트 파일 제외
  - "internal/**/*.go"  # internal/ 하위 .go 파일만
```

## 권한 검증

Skill은 실행 전 권한을 검증받아야 합니다:

```go
type PermissionChecker interface {
    CheckPermission(skill *SkillDefinition, user string) error
}

// @MX:SECURITY 모든 skill 실행 전 권한 검증 필수
func (r *SkillRegistry) Execute(id string, user string) error {
    skill, ok := r.Get(id)
    if !ok {
        return ErrSkillNotFound
    }

    if err := r.permChecker.CheckPermission(skill, user); err != nil {
        return err
    }

    // ... skill 실행
}
```

## QueryEngine 연계

`QUERY-001`의 `QueryEngine`은 매 iteration 직전 Skill을 로드:

```go
func (qe *QueryEngine) prepareSkills(ctx context.Context, paths []string) error {
    // 1. paths에 매칭되는 Skill 로드
    skills := qe.skillRegistry.MatchedFiles(paths)

    // 2. Progressive Disclosure에 따라 필터링
    skills = filterByEffort(skills, qe.maxEffort)

    // 3. system/user prompt에 Skill 내용 반영
    for _, skill := range skills {
        qe.systemPrompt = applySkill(qe.systemPrompt, skill)
        qe.userPrompt = applySkill(qe.userPrompt, skill)
    }

    return nil
}
```

## 테스트

### 단위 테스트

```bash
go test ./internal/skill/...
```

현재 테스트 커버리지: **86.7%** (29 테스트 함수, race-clean)

### 테스트 데이터

```
internal/skill/testdata/
├── skill_valid.md           # 유효한 skill 예시
├── skill_invalid_yaml.md    # YAML 파싱 실패 케이스
├── skill_conditional.md     # 조건부 skill 예시
└── paths/                   # paths 매칭 테스트 케이스
    ├── exact.go
    ├── pattern.go
    └── negation.go
```

## 파일 구조

```
internal/skill/
├── schema.go              # SkillDefinition, Frontmatter 구조체
├── parser.go              # YAML frontmatter 파서
├── runtime.go             # Skill 실행 런타임
├── conditional.go         # 조건부 로직 평가
├── loader.go              # 디스크 skill 로더
├── remote.go              # 원격 URL skill 로더
├── registry.go            # Skill 레지스트리
├── testdata/              # 테스트 fixture
│   ├── skill_valid.md
│   ├── skill_invalid_yaml.md
│   ├── skill_conditional.md
│   └── paths/
└── *_test.go              # 각 구성 요소별 테스트
```

## 상호 의존성

본 패키지는 다음 SPEC와 통합됩니다:

- **QUERY-001**: `QueryEngine`이 매 iteration 직전 Skill 로드
- **SUBAGENT-001**: agent system prompt를 forked skill로 구성
- **PLUGIN-001**: plugin manifest의 `skills:` 배열 로드
- **PERMISSION-001**: Skill 실행 전 권한 검증

## 관련 SPEC

- **SPEC-GOOSE-SKILLS-001**: 본 패키지의 주요 SPEC (P0, Phase 2)
- **SPEC-GOOSE-QUERY-001**: QueryEngine iteration 전 Skill 로드 훅 포인트
- **SPEC-GOOSE-SUBAGENT-001**: Forked skill 기반 agent prompt
- **SPEC-GOOSE-PLUGIN-001**: Plugin manifest skills 배열 로드

---

Version: 0.3.1
Last Updated: 2026-04-27
SPEC: SPEC-GOOSE-SKILLS-001
