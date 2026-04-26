// Package skill은 AI.GOOSE의 Skill 시스템을 구현한다.
// YAML frontmatter 파싱, 4-trigger 활성화, Progressive Disclosure L0~L3,
// 변수 치환, gitignore 기반 조건부 매칭, allowlist-default-deny 보안 게이트를 제공한다.
//
// 표준 정렬: agentskills.io Claude Code Agent Skills 표준의 compatible subset + MoAI-ADK 보안 레이어.
// shell: parse-time 실행, 임의 env var 치환은 agentskills.io보다 엄격하게 금지한다 (REQ-SK-013/014/022).
//
// SPEC: SPEC-GOOSE-SKILLS-001 v0.3.0
package skill

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// TriggerMode는 skill 활성화 방식을 나타내는 discriminated union이다.
// REQ-SK-021: 정확히 4종 trigger만 지원한다.
type TriggerMode int

const (
	// TriggerInline은 기본 trigger — QueryEngine prompt에 직접 주입된다.
	TriggerInline TriggerMode = iota
	// TriggerFork는 context: fork 설정 시 — SUBAGENT-001이 별도 agent로 실행한다.
	TriggerFork
	// TriggerConditional은 paths: 설정 시 — FileChangedConsumer에 의해 활성화된다.
	TriggerConditional
	// TriggerRemote는 _canonical_ 접두사 ID — HTTP fetch로 로드된다.
	TriggerRemote
)

// EffortLevel은 Progressive Disclosure의 L0~L3 토큰 예산 티어를 나타낸다.
// L0~L3은 저자가 선언한 스킬의 분량을 나타내며,
// MoAI-ADK 3-Level(Metadata/Body/Bundled)과는 독립적인 별도 축이다 (REQ-SK-020).
type EffortLevel int

const (
	// EffortL0은 약 50 tokens 수준의 최소 skill이다.
	EffortL0 EffortLevel = iota
	// EffortL1은 약 200 tokens 수준의 기본 skill이다 (기본값).
	EffortL1
	// EffortL2는 약 500 tokens 수준의 상세 skill이다.
	EffortL2
	// EffortL3은 약 1000+ tokens 수준의 풍부한 skill이다.
	EffortL3
)

// SkillFrontmatter는 SKILL.md의 YAML frontmatter 스키마다.
// 알려진 15개 속성만 필드로 존재한다 (REQ-SK-019).
// 알 수 없는 속성은 allowlist-default-deny로 거부된다 (REQ-SK-001).
type SkillFrontmatter struct {
	// name: skill의 고유 식별자 (path-derived slug로 ID 파생)
	Name string `yaml:"name,omitempty"`
	// description: skill의 한 줄 설명
	Description string `yaml:"description,omitempty"`
	// when-to-use: 이 skill을 사용할 시점 (MoAI-ADK 확장)
	WhenToUse string `yaml:"when-to-use,omitempty"`
	// argument-hint: slash-command autocomplete hint (REQ-SK-018)
	ArgumentHint string `yaml:"argument-hint,omitempty"`
	// arguments: 허용된 인수 목록
	Arguments []string `yaml:"arguments,omitempty"`
	// model: 선호 모델 alias (REQ-SK-017, consumer ROUTER-001이 honor/override 결정)
	Model string `yaml:"model,omitempty"`
	// effort: L0|L1|L2|L3 또는 정수 0~3 (REQ-SK-010, 기본 L1)
	Effort string `yaml:"effort,omitempty"`
	// context: "fork" 설정 시 TriggerFork로 판정 (REQ-SK-008)
	Context string `yaml:"context,omitempty"`
	// agent: 연관 agent 식별자 (parse만, 실행은 SUBAGENT-001)
	Agent string `yaml:"agent,omitempty"`
	// allowed-tools: 허용된 도구 목록
	AllowedTools []string `yaml:"allowed-tools,omitempty"`
	// disable-model-invocation: true이면 model actor의 CanInvoke가 false (REQ-SK-009)
	DisableModelInvocation bool `yaml:"disable-model-invocation,omitempty"`
	// user-invocable: nil=기본 true, *false=사용자 호출 불가 (REQ-SK-009)
	UserInvocable *bool `yaml:"user-invocable,omitempty"`
	// paths: gitignore 문법의 조건부 매칭 패턴 목록 (REQ-SK-007)
	Paths []string `yaml:"paths,omitempty"`
	// shell: shell 설정 (parse-time 실행 금지, REQ-SK-013, REQ-SK-022)
	Shell *SkillShellConfig `yaml:"shell,omitempty"`
	// hooks: 이벤트 핸들러 맵 (배열 순서 보존, parse만, REQ-SK-002, REQ-SK-022)
	Hooks map[string][]SkillHookEntry `yaml:"hooks,omitempty"`
}

// SkillShellConfig는 shell 실행 설정을 담는다.
// 파서는 이 값을 리터럴로만 기록하며 parse-time에 실행하지 않는다 (REQ-SK-013, REQ-SK-022a).
type SkillShellConfig struct {
	// Executable은 shell 실행 경로 (리터럴 보존, PATH 조회·EvalSymlinks 금지)
	Executable string `yaml:"executable"`
	// DenyWrite는 consumer에게 sandboxed execution을 힌트하는 플래그 (REQ-SK-022b)
	DenyWrite bool `yaml:"deny-write,omitempty"`
}

// SkillHookEntry는 단일 hook 이벤트 핸들러를 담는다.
// command는 raw 문자열로 보존되며, 파서는 shell metacharacter를 변환하지 않는다 (REQ-SK-022c).
type SkillHookEntry struct {
	// Matcher는 hook 매처 (선택적)
	Matcher string `yaml:"matcher,omitempty"`
	// Command는 실행할 명령어 (raw 문자열 보존, escape/sanitization 금지)
	Command string `yaml:"command"`
}

// SkillDefinition은 레지스트리가 유지하는 완전한 skill 표현이다.
type SkillDefinition struct {
	// ID는 path-derived slug (name 필드 기반)
	ID string
	// AbsolutePath는 SKILL.md의 절대 경로
	AbsolutePath string
	// Frontmatter는 파싱된 YAML frontmatter
	Frontmatter SkillFrontmatter
	// Body는 치환 전 raw body 문자열
	Body string
	// Trigger는 결정된 활성화 방식 (REQ-SK-021)
	Trigger TriggerMode
	// Effort는 저자가 선언한 Progressive Disclosure 티어 (REQ-SK-010, REQ-SK-020)
	Effort EffortLevel
	// PreferredModel은 frontmatter model 필드의 리터럴 (REQ-SK-017)
	PreferredModel string
	// FrontmatterTokens는 EstimateSkillFrontmatterTokens 결과 캐시
	FrontmatterTokens int
	// IsRemote는 HTTP로 로드된 remote skill 여부 (REQ-SK-012)
	IsRemote bool
	// ArgumentHint는 argument-hint 필드의 리터럴 (REQ-SK-018)
	ArgumentHint string
}

// SAFE_SKILL_PROPERTIES는 허용된 YAML frontmatter 키의 allowlist다.
// REQ-SK-019: 정확히 15개의 키만 허용한다.
// 새 키 추가는 semver-minor 버전 업 + 테스트 업데이트 + SPEC HISTORY 갱신이 필요하다.
//
// @MX:ANCHOR: [AUTO] SAFE_SKILL_PROPERTIES — allowlist-default-deny 보안 게이트의 핵심 상수
// @MX:REASON: REQ-SK-001/019 — 모든 frontmatter 파싱의 allowlist 기준점. fan_in >= 3 (parser/validator/test)
// @MX:SPEC: REQ-SK-001, REQ-SK-019
var SAFE_SKILL_PROPERTIES = map[string]struct{}{
	"name":                     {},
	"description":              {},
	"when-to-use":              {},
	"argument-hint":            {},
	"arguments":                {},
	"model":                    {},
	"effort":                   {},
	"context":                  {},
	"agent":                    {},
	"allowed-tools":            {},
	"disable-model-invocation": {},
	"user-invocable":           {},
	"paths":                    {},
	"shell":                    {},
	"hooks":                    {},
}

// --- 에러 타입 ---

// ErrUnsafeFrontmatterProperty는 allowlist에 없는 YAML 키가 발견될 때 반환된다.
// REQ-SK-001, REQ-SK-019
type ErrUnsafeFrontmatterProperty struct {
	// Property는 거부된 frontmatter 키 이름이다.
	Property string
}

func (e ErrUnsafeFrontmatterProperty) Error() string {
	return fmt.Sprintf("unsafe frontmatter property: %q is not in SAFE_SKILL_PROPERTIES allowlist", e.Property)
}

func (e ErrUnsafeFrontmatterProperty) Is(target error) bool {
	var t ErrUnsafeFrontmatterProperty
	return errors.As(target, &t) && t.Property == e.Property
}

// ErrSymlinkEscape는 root 외부를 가리키는 symlink가 발견될 때 error slice에 추가된다.
// REQ-SK-015
type ErrSymlinkEscape struct {
	// Path는 문제의 symlink 경로다.
	Path string
}

func (e ErrSymlinkEscape) Error() string {
	return fmt.Sprintf("symlink escape: %q resolves outside root directory", e.Path)
}

// ErrDuplicateSkillID는 같은 ID의 skill이 중복 발견될 때 error slice에 추가된다.
// REQ-SK-004: 첫 번째 등록 항목은 보존되고, 중복 항목은 error slice에만 기록된다.
type ErrDuplicateSkillID struct {
	// ID는 중복된 skill ID다.
	ID string
	// Path는 나중에 발견된(거부된) 파일 경로다.
	Path string
}

func (e ErrDuplicateSkillID) Error() string {
	return fmt.Sprintf("duplicate skill ID: %q already registered, offending file: %s", e.ID, e.Path)
}

// isSymlinkEscapeErr는 error가 ErrSymlinkEscape인지 확인한다 (테스트 헬퍼용).
func isSymlinkEscapeErr(err error, target *ErrSymlinkEscape) bool {
	return errors.As(err, target)
}

// isDuplicateSkillIDErr는 error가 ErrDuplicateSkillID인지 확인한다 (테스트 헬퍼용).
func isDuplicateSkillIDErr(err error, target *ErrDuplicateSkillID) bool {
	return errors.As(err, target)
}

// --- SkillRegistry ---

// SkillRegistry는 로드된 skill을 관리하는 컨테이너다.
// atomic Replace로 hot-reload 지원하며, sync.RWMutex로 concurrent read를 보호한다.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]*SkillDefinition
	logger *zap.Logger
}

// NewSkillRegistry는 비어있는 SkillRegistry를 생성한다.
func NewSkillRegistry(logger *zap.Logger) *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]*SkillDefinition),
		logger: logger,
	}
}

// Get은 id에 해당하는 SkillDefinition을 반환한다.
// 없으면 (nil, false)를 반환한다.
func (r *SkillRegistry) Get(id string) (*SkillDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.skills[id]
	return def, ok
}

// Replace는 외부 map을 deep copy하여 내부 skills map을 atomic swap한다.
// REQ-SK-016: in-place mutation 없이 새 map으로 교체, caller mutation 격리.
//
// @MX:WARN: [AUTO] sync.RWMutex 기반 atomic swap — race detector 검증 필요
// @MX:REASON: concurrent reader 100개 + 1 writer 시나리오에서 혼합 상태가 없어야 함 (AC-SK-014)
func (r *SkillRegistry) Replace(newSkills map[string]*SkillDefinition) {
	// 외부 map을 deep copy하여 caller mutation 격리
	copied := make(map[string]*SkillDefinition, len(newSkills))
	for k, v := range newSkills {
		copied[k] = v
	}

	r.mu.Lock()
	r.skills = copied
	r.mu.Unlock()
}

// replaceInternal은 테스트 전용 internal replace다 (deep copy 없이 직접 대입).
func (r *SkillRegistry) replaceInternal(newSkills map[string]*SkillDefinition) {
	r.mu.Lock()
	r.skills = newSkills
	r.mu.Unlock()
}

// CanInvoke는 actor가 id skill을 호출할 수 있는지 3-branch matrix로 판단한다.
// REQ-SK-009:
//   - model actor: disable-model-invocation이 true이면 false
//   - user actor: user-invocable이 명시된 경우 그 값, 아니면 기본 true
//   - hook actor: disable-model-invocation과 무관하게 true (model 전용 차단)
//   - unknown actor: 항상 false (default-deny)
//
// @MX:ANCHOR: [AUTO] CanInvoke — 모든 skill 호출 권한 게이트
// @MX:REASON: REQ-SK-009 3-branch actor matrix — user/model/hook/unknown 판단의 단일 진입점
// @MX:SPEC: REQ-SK-009
func (r *SkillRegistry) CanInvoke(id string, actor string) bool {
	def, ok := r.Get(id)
	if !ok {
		return false
	}
	fm := def.Frontmatter

	switch actor {
	case "user":
		// user-invocable 명시 여부 확인
		if fm.UserInvocable != nil {
			return *fm.UserInvocable
		}
		return true // 기본 true

	case "model":
		return !fm.DisableModelInvocation

	case "hook":
		// hook는 disable-model-invocation과 무관하게 허용
		// (model 전용 차단이며 hook/user는 별도 제어)
		return true

	default:
		// 알 수 없는 actor는 default-deny (REQ-SK-009c, REQ-SK-001과 일관성)
		return false
	}
}

// FileChangedConsumer는 변경된 파일 경로를 받아 활성화 대상 skill ID 목록을 반환한다.
// REQ-SK-007: 조건부 skill의 paths 패턴을 gitignore 문법으로 매칭한다.
// paths 미지정 skill은 어떤 changedPaths에도 매칭되지 않는다 (REQ-SK-011).
//
// @MX:ANCHOR: [AUTO] FileChangedConsumer — HOOK-001의 cross-package consumer 표면
// @MX:REASON: FileChanged 이벤트가 이 함수를 통해 skill 활성화로 라우팅된다 (REQ-SK-007, REQ-SK-011)
// @MX:SPEC: REQ-SK-007, REQ-SK-011
func (r *SkillRegistry) FileChangedConsumer(changed []string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []string
	seen := make(map[string]bool)

	for id, def := range r.skills {
		// paths 미지정 skill은 제외 (REQ-SK-011)
		if len(def.Frontmatter.Paths) == 0 {
			continue
		}

		if matchesPaths(def.Frontmatter.Paths, changed) && !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}

	return result
}

// --- 헬퍼 함수 ---

// validateFrontmatterMap은 raw map의 키를 SAFE_SKILL_PROPERTIES allowlist로 검증한다.
// REQ-SK-001: 알 수 없는 키 발견 시 ErrUnsafeFrontmatterProperty를 반환한다.
func validateFrontmatterMap(raw map[string]any) error {
	for key := range raw {
		if _, ok := SAFE_SKILL_PROPERTIES[key]; !ok {
			return ErrUnsafeFrontmatterProperty{Property: key}
		}
	}
	return nil
}

// parseFrontmatter는 YAML bytes를 2단계 unmarshal로 파싱한다.
// §6.7: 1단계 map[string]any → 키 검증 → 2단계 SkillFrontmatter
func parseFrontmatter(data []byte) (SkillFrontmatter, error) {
	// 1단계: loose unmarshal로 키 목록 추출
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return SkillFrontmatter{}, fmt.Errorf("YAML 파싱 실패: %w", err)
	}

	// allowlist 검증
	if err := validateFrontmatterMap(raw); err != nil {
		return SkillFrontmatter{}, err
	}

	// 2단계: struct unmarshal
	var fm SkillFrontmatter
	if err := yaml.Unmarshal(data, &fm); err != nil {
		return SkillFrontmatter{}, fmt.Errorf("frontmatter struct 파싱 실패: %w", err)
	}

	return fm, nil
}
