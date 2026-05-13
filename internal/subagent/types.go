// Package subagent는 MINK의 Sub-agent 런타임을 구현한다.
// 3종 isolation(fork / worktree / background) + 3-scope memory(user / project / local)
// + role profile override를 제공한다.
//
// SPEC-GOOSE-SUBAGENT-001 v0.3.0
package subagent

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/query"
)

// IsolationMode는 sub-agent 격리 모드를 나타낸다.
// REQ-SA-005/006/007
type IsolationMode string

const (
	// IsolationFork는 부모 컨텍스트 상속 + 독립 token budget + TeammateIdentity 부여.
	IsolationFork IsolationMode = "fork"
	// IsolationWorktree는 git worktree 생성 + CWD 격리 + WorktreeCreate hook 발동.
	IsolationWorktree IsolationMode = "worktree"
	// IsolationBackground는 동일 프로세스, 별도 goroutine, non-blocking.
	IsolationBackground IsolationMode = "background"
)

// MemoryScope는 3-scope 메모리 디렉토리 중 하나를 나타낸다.
// REQ-SA-003
type MemoryScope string

const (
	// ScopeUser는 사용자 홈 디렉토리 기반 메모리 스코프이다.
	// ~/.mink/agent-memory/{agentType}/
	ScopeUser MemoryScope = "user"
	// ScopeProject는 프로젝트 루트 기반 메모리 스코프이다.
	// {projectRoot}/.mink/agent-memory/{agentType}/
	ScopeProject MemoryScope = "project"
	// ScopeLocal는 git-ignored 로컬 메모리 스코프이다.
	// {projectRoot}/.mink/agent-memory-local/{agentType}/
	ScopeLocal MemoryScope = "local"
)

// SubagentState는 sub-agent의 생명주기 상태이다.
// REQ-SA-008
type SubagentState int

const (
	// StatePending은 spawn 전 초기 상태이다.
	StatePending SubagentState = iota
	// StateRunning은 실행 중 상태이다.
	StateRunning
	// StateCompleted는 성공 완료 상태이다.
	StateCompleted
	// StateFailed는 실패 완료 상태이다.
	StateFailed
	// StateIdle은 background isolation의 비활성 상태이다.
	StateIdle
)

// DefaultBackgroundIdleThreshold는 background agent의 기본 유휴 임계값이다.
// REQ-SA-007: 5초 동안 새 메시지 없으면 TeammateIdle 이벤트 발동.
const DefaultBackgroundIdleThreshold = 5 * time.Second

// GoroutineShutdownGrace는 goroutine이 ctx.Done() 후 종료될 때까지의 유예 시간이다.
// REQ-SA-023
const GoroutineShutdownGrace = 100 * time.Millisecond

// MaxSpawnDepth는 cyclic spawn 방지를 위한 최대 중첩 깊이이다.
// REQ-SA-014
const MaxSpawnDepth = 5

// PlanApprovalTimeout은 plan mode 승인 대기 최대 시간이다.
// REQ-SA-022(d)
const PlanApprovalTimeout = 300 * time.Second

// agentIDDelimiter는 AgentID에서 agentName과 세션 정보를 구분하는 구분자이다.
// REQ-SA-001, REQ-SA-018: '@'는 예약 구분자
const agentIDDelimiter = "@"

// AgentDefinition은 .claude/agents/{name}.md의 frontmatter 파싱 결과이다.
// REQ-SA-004: SkillFrontmatter allowlist 검증 동일.
//
// @MX:ANCHOR: [AUTO] sub-agent 스폰의 단일 설정 컨테이너
// @MX:REASON: SPEC-GOOSE-SUBAGENT-001 REQ-SA-004/005 — 모든 spawn 경로가 이 타입을 소비한다
type AgentDefinition struct {
	// AgentType은 파일명 기반 slug이다 (e.g., "researcher", "analyst").
	AgentType string `yaml:"agent_type,omitempty"`
	// Name은 frontmatter name 필드이다.
	Name string `yaml:"name,omitempty"`
	// Description은 에이전트 설명이다.
	Description string `yaml:"description,omitempty"`
	// Tools는 허용 tool 목록이다. ["*"]이면 부모 상속.
	Tools []string `yaml:"allowed-tools,omitempty"`
	// UseExactTools는 Tools 목록을 정확히 사용할지 여부이다.
	UseExactTools bool `yaml:"use-exact-tools,omitempty"`
	// Model은 모델 alias이다. "inherit"이면 부모 모델 사용.
	Model string `yaml:"model,omitempty"`
	// MaxTurns는 최대 turn 수이다.
	MaxTurns int `yaml:"max-turns,omitempty"`
	// PermissionMode는 권한 모드이다 ("bubble" | "isolated" | "plan").
	PermissionMode string `yaml:"permission-mode,omitempty"`
	// Effort는 노력 수준이다 (L0/L1/L2/L3).
	Effort string `yaml:"effort,omitempty"`
	// SystemPrompt는 markdown body에서 파싱된 system prompt이다.
	SystemPrompt string `yaml:"-"`
	// MCPServers는 MCP-001 ConnectToServer 호출 대상 목록이다.
	MCPServers []string `yaml:"mcp-servers,omitempty"`
	// MemoryScopes는 memory.append tool이 접근할 수 있는 스코프 목록이다.
	// REQ-SA-021: non-empty이면 memory.append 빌트인 tool이 등록된다.
	MemoryScopes []MemoryScope `yaml:"memory-scopes,omitempty"`
	// Isolation은 격리 모드이다.
	Isolation IsolationMode `yaml:"isolation,omitempty"`
	// Source는 정의 출처이다 ("user" | "plugin" | "builtin").
	Source string `yaml:"source,omitempty"`
	// Background는 Isolation=Background의 단축키이다.
	Background bool `yaml:"background,omitempty"`
	// CoordinatorMode는 coordinator 모드 활성화 여부이다.
	CoordinatorMode bool `yaml:"coordinator-mode,omitempty"`
}

// TeammateIdentity는 teammate 모드에서 context에 주입되는 식별 정보이다.
// REQ-SA-005(b)
type TeammateIdentity struct {
	// AgentID는 에이전트 식별자이다. 형식: "{agentName}@{sessionId}-{spawnIndex}"
	AgentID string
	// AgentName은 에이전트 이름이다.
	AgentName string
	// TeamName은 팀 이름이다.
	TeamName string
	// PlanModeRequired는 plan mode 승인 대기 여부이다.
	// REQ-SA-022(a)
	PlanModeRequired bool
	// ParentSessionID는 부모 세션 식별자이다.
	ParentSessionID string
}

// teammateContextKey는 context.WithValue의 key 타입이다.
// 타입 충돌 방지를 위해 별도 타입 사용.
type teammateContextKey struct{}

// spawnDepthKey는 spawn depth를 context에 저장하는 key 타입이다.
// REQ-SA-014: cyclic spawn 방지.
type spawnDepthKey struct{}

// WithTeammateIdentity는 TeammateIdentity를 context에 주입한다.
// REQ-SA-005(b): context.WithValue를 통한 AsyncLocalStorage 대체.
func WithTeammateIdentity(ctx context.Context, id TeammateIdentity) context.Context {
	return context.WithValue(ctx, teammateContextKey{}, id)
}

// TeammateIdentityFromContext는 context에서 TeammateIdentity를 추출한다.
func TeammateIdentityFromContext(ctx context.Context) (TeammateIdentity, bool) {
	v, ok := ctx.Value(teammateContextKey{}).(TeammateIdentity)
	return v, ok
}

// spawnDepthFromContext는 context에서 spawn depth를 추출한다.
func spawnDepthFromContext(ctx context.Context) int {
	v, ok := ctx.Value(spawnDepthKey{}).(int)
	if !ok {
		return 0
	}
	return v
}

// withSpawnDepth는 spawn depth를 1 증가시켜 context에 저장한다.
func withSpawnDepth(ctx context.Context) context.Context {
	depth := spawnDepthFromContext(ctx)
	return context.WithValue(ctx, spawnDepthKey{}, depth+1)
}

// Subagent는 실행 중인 sub-agent 인스턴스이다.
type Subagent struct {
	// AgentID는 에이전트 고유 식별자이다.
	AgentID string
	// Definition은 에이전트 정의이다.
	Definition AgentDefinition
	// Engine은 내부 QueryEngine 인스턴스이다.
	Engine *query.QueryEngine
	// state는 atomic으로 관리되는 생명주기 상태이다.
	// REQ-SA-008: state mutation은 channel close happen-before.
	state int64
	// Identity는 teammate 식별 정보이다.
	Identity TeammateIdentity
	// MemoryDir은 선택된 scope의 디렉토리이다.
	MemoryDir string
	// StartedAt은 spawn 시각이다.
	StartedAt time.Time
	// FinishedAt은 완료 시각이다. nil이면 미완료.
	FinishedAt *time.Time
}

// State는 Subagent의 현재 상태를 반환한다.
func (s *Subagent) State() SubagentState {
	return SubagentState(atomic.LoadInt64(&s.state))
}

// setState는 Subagent의 상태를 atomic으로 설정한다.
func (s *Subagent) setState(st SubagentState) {
	atomic.StoreInt64(&s.state, int64(st))
}

// SubagentInput은 RunAgent 호출 시 전달되는 입력이다.
type SubagentInput struct {
	// Prompt는 초기 프롬프트이다.
	Prompt string
	// InitialMessages는 부모 context 상속 시 복사되는 메시지 목록이다.
	InitialMessages []message.Message
	// Metadata는 추가 메타데이터이다.
	Metadata map[string]any
}

// MemoryEntry는 memdir.jsonl 항목이다.
// REQ-SA-021
type MemoryEntry struct {
	ID        string      `json:"id"`
	Timestamp time.Time   `json:"ts"`
	Category  string      `json:"category"`
	Key       string      `json:"key"`
	Value     any         `json:"value"`
	Scope     MemoryScope `json:"scope,omitempty"`
}

// 에러 sentinel 정의

var (
	// ErrUnsafeAgentProperty는 frontmatter에 허용되지 않은 속성이 있을 때 반환된다.
	// REQ-SA-004 / AC-SA-013
	ErrUnsafeAgentProperty = errors.New("subagent: unsafe agent property in frontmatter")

	// ErrInvalidAgentName은 에이전트 이름이 유효하지 않을 때 반환된다.
	// REQ-SA-018 / AC-SA-016
	ErrInvalidAgentName = errors.New("subagent: invalid agent name")

	// ErrEngineInitFailed는 QueryEngine 생성 실패 시 반환된다.
	// REQ-SA-005-F(i)
	ErrEngineInitFailed = errors.New("subagent: failed to initialize QueryEngine")

	// ErrHookDispatchFailed는 hook dispatch 실패 시 반환된다.
	// REQ-SA-005-F(ii)
	ErrHookDispatchFailed = errors.New("subagent: hook dispatch failed")

	// ErrSpawnAborted는 goroutine spawn 실패 시 반환된다.
	// REQ-SA-005-F(iii)
	ErrSpawnAborted = errors.New("subagent: spawn aborted")

	// ErrSpawnDepthExceeded는 spawn depth가 MaxSpawnDepth를 초과했을 때 반환된다.
	// REQ-SA-014 / AC-SA-009
	ErrSpawnDepthExceeded = errors.New("subagent: spawn depth exceeded maximum")

	// ErrAgentNotFound는 활성 sub-agent를 찾을 수 없을 때 반환된다.
	// PlanModeApprove
	ErrAgentNotFound = errors.New("subagent: agent not found")

	// ErrAgentNotInPlanMode는 agent가 plan mode가 아닐 때 반환된다.
	// PlanModeApprove
	ErrAgentNotInPlanMode = errors.New("subagent: agent is not in plan mode")

	// ErrMemdirLockUnsupported는 파일시스템이 advisory locking을 지원하지 않을 때 반환된다.
	// REQ-SA-012(c) / AC-SA-014
	ErrMemdirLockUnsupported = errors.New("subagent: filesystem does not support advisory locking")

	// ErrScopeNotEnabled는 sub-agent의 enabled scopes에 없는 scope에 접근할 때 반환된다.
	// REQ-SA-021 / AC-SA-019
	ErrScopeNotEnabled = errors.New("subagent: memory scope is not enabled for this agent")

	// ErrTranscriptCorrupted는 transcript 로드 실패 시 반환된다.
	// R5 risk
	ErrTranscriptCorrupted = errors.New("subagent: transcript is corrupted or unreadable")

	// ErrUnknownModelAlias는 model alias 해석 실패 시 반환된다.
	// REQ-SA-019 / AC-SA-017
	ErrUnknownModelAlias = errors.New("subagent: unknown model alias")
)
