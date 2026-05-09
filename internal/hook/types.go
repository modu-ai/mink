// Package hook는 AI.GOOSE의 24개 lifecycle hook 이벤트 디스패처와
// useCanUseTool 권한 결정 플로우를 구현한다.
// SPEC-GOOSE-HOOK-001
package hook

import (
	"context"
	"errors"
)

// HookEvent는 29개 HookEvent 상수 (24 base + 5 ritual SCHEDULER-001)를 나타낸다.
// AC-HK-001 / REQ-HK-001
type HookEvent string

// 29개 HookEvent 상수 (24 base + 5 ritual SCHEDULER-001).
// Base 24: claude-primitives §5.1 기반; Elicitation, ElicitationResult, InstructionsLoaded 제외 (AC-HK-001).
// Ritual 5: SPEC-GOOSE-SCHEDULER-001 P1 추가.
const (
	EvSetup              HookEvent = "Setup"
	EvSessionStart       HookEvent = "SessionStart"
	EvSubagentStart      HookEvent = "SubagentStart"
	EvUserPromptSubmit   HookEvent = "UserPromptSubmit"
	EvPreToolUse         HookEvent = "PreToolUse"
	EvPostToolUse        HookEvent = "PostToolUse"
	EvPostToolUseFailure HookEvent = "PostToolUseFailure"
	EvCwdChanged         HookEvent = "CwdChanged"
	EvFileChanged        HookEvent = "FileChanged"
	EvWorktreeCreate     HookEvent = "WorktreeCreate"
	EvWorktreeRemove     HookEvent = "WorktreeRemove"
	EvPermissionRequest  HookEvent = "PermissionRequest"
	EvPermissionDenied   HookEvent = "PermissionDenied"
	EvNotification       HookEvent = "Notification"
	EvPreCompact         HookEvent = "PreCompact"
	EvPostCompact        HookEvent = "PostCompact"
	EvStop               HookEvent = "Stop"
	EvStopFailure        HookEvent = "StopFailure"
	EvSubagentStop       HookEvent = "SubagentStop"
	EvTeammateIdle       HookEvent = "TeammateIdle"
	EvTaskCreated        HookEvent = "TaskCreated"
	EvTaskCompleted      HookEvent = "TaskCompleted"
	EvSessionEnd         HookEvent = "SessionEnd"
	EvConfigChange       HookEvent = "ConfigChange"
	// Ritual events added by SPEC-GOOSE-SCHEDULER-001 P1.
	EvMorningBriefingTime HookEvent = "MorningBriefingTime"
	EvPostBreakfastTime   HookEvent = "PostBreakfastTime"
	EvPostLunchTime       HookEvent = "PostLunchTime"
	EvPostDinnerTime      HookEvent = "PostDinnerTime"
	EvEveningCheckInTime  HookEvent = "EveningCheckInTime"
)

// HookEventNames는 29개 HookEvent 상수 (24 base + 5 ritual SCHEDULER-001)의 문자열 집합을 반환한다.
// AC-HK-001 검증 함수.
func HookEventNames() []string {
	return []string{
		string(EvSetup),
		string(EvSessionStart),
		string(EvSubagentStart),
		string(EvUserPromptSubmit),
		string(EvPreToolUse),
		string(EvPostToolUse),
		string(EvPostToolUseFailure),
		string(EvCwdChanged),
		string(EvFileChanged),
		string(EvWorktreeCreate),
		string(EvWorktreeRemove),
		string(EvPermissionRequest),
		string(EvPermissionDenied),
		string(EvNotification),
		string(EvPreCompact),
		string(EvPostCompact),
		string(EvStop),
		string(EvStopFailure),
		string(EvSubagentStop),
		string(EvTeammateIdle),
		string(EvTaskCreated),
		string(EvTaskCompleted),
		string(EvSessionEnd),
		string(EvConfigChange),
		// Ritual events added by SPEC-GOOSE-SCHEDULER-001 P1.
		string(EvMorningBriefingTime),
		string(EvPostBreakfastTime),
		string(EvPostLunchTime),
		string(EvPostDinnerTime),
		string(EvEveningCheckInTime),
	}
}

// ToolInfo는 tool 관련 이벤트에서 사용되는 tool 메타데이터이다.
type ToolInfo struct {
	Name  string
	Input map[string]any
}

// HookError는 PostToolUseFailure 이벤트에서 사용되는 오류 정보이다.
type HookError struct {
	Message string
	Code    string
}

// HookInput은 모든 이벤트가 공유하는 공통 입력 구조이다.
// REQ-HK-014: 디스패처는 각 핸들러에 독립적인 deep copy를 전달해야 한다.
type HookInput struct {
	HookEvent    HookEvent
	ToolUseID    string    // tool 관련 이벤트에서만
	Tool         *ToolInfo // PreToolUse/PostToolUse
	Input        map[string]any
	Output       any        // PostToolUse만
	Error        *HookError // PostToolUseFailure만
	ChangedPaths []string   // FileChanged만
	SessionID    string
	CustomData   map[string]any
}

// PermissionDecision은 hook이 반환하는 권한 결정이다.
type PermissionDecision struct {
	Approve bool
	Reason  string
	Details map[string]any
}

// HookJSONOutput은 shell command hook의 stdout JSON 출력 구조이다.
// REQ-HK-006: exit code 0 시 JSON 파싱, exit code 2 시 blocking signal.
type HookJSONOutput struct {
	Continue             *bool               `json:"continue,omitempty"`
	SuppressOutput       bool                `json:"suppressOutput,omitempty"`
	Async                bool                `json:"async,omitempty"`
	AsyncTimeout         int                 `json:"asyncTimeout,omitempty"`
	PermissionDecision   *PermissionDecision `json:"permissionDecision,omitempty"`
	InitialUserMessage   string              `json:"initialUserMessage,omitempty"`
	WatchPaths           []string            `json:"watchPaths,omitempty"`
	AdditionalContext    string              `json:"additionalContext,omitempty"`
	UpdatedMCPToolOutput any                 `json:"updatedMCPToolOutput,omitempty"`
}

// ptrBool은 bool 포인터를 생성하는 헬퍼이다.
func ptrBool(b bool) *bool { return &b }

// HookHandler는 단일 hook 핸들러 인터페이스이다.
// REQ-HK-002: 등록 순서(FIFO)로 호출된다.
type HookHandler interface {
	// Handle은 hook 이벤트를 처리한다.
	Handle(ctx context.Context, input HookInput) (HookJSONOutput, error)
	// Matches는 이 핸들러가 주어진 input에 매치되는지 반환한다.
	// REQ-HK-020: matcher 시스템 (glob 또는 regex: 접두사)
	Matches(input HookInput) bool
}

// HookBinding은 HookRegistry 내부의 등록 항목이다.
type HookBinding struct {
	Event   HookEvent
	Matcher string // glob 또는 "regex:" 접두사
	Handler HookHandler
	Source  string // "inline" | "plugin" | "builtin"
}

// PermissionBehavior는 useCanUseTool 결정 결과이다.
// REQ-HK-009
type PermissionBehavior int

const (
	// PermAllow는 tool 즉시 실행을 허가한다.
	PermAllow PermissionBehavior = iota
	// PermDeny는 tool 실행을 거부한다.
	PermDeny
	// PermAsk는 외부 결정을 대기한다.
	PermAsk
)

// String은 PermissionBehavior의 문자열 표현을 반환한다.
func (b PermissionBehavior) String() string {
	switch b {
	case PermAllow:
		return "allow"
	case PermDeny:
		return "deny"
	case PermAsk:
		return "ask"
	default:
		return "unknown"
	}
}

// DecisionReason은 권한 결정 이유를 담는 구조이다.
type DecisionReason struct {
	Type    string // "yolo_auto" | "handler" | "interactive" | "coordinator" | "swarm" | "no_interactive_fallback"
	Reason  string
	Details map[string]any
}

// PermissionResult는 useCanUseTool의 반환 타입이다.
type PermissionResult struct {
	Behavior       PermissionBehavior
	DecisionReason *DecisionReason
}

// Role은 permCtx.Role 값 집합이다.
// D12 resolution: SUBAGENT-001과 CLI-001이 사용하는 역할 enum.
type Role string

const (
	// RoleCoordinator는 SUBAGENT-001 coordinator 모드이다.
	RoleCoordinator Role = "coordinator"
	// RoleSwarmWorker는 SUBAGENT-001 swarm worker 모드이다.
	RoleSwarmWorker Role = "swarm_worker"
	// RoleInteractive는 사용자 터미널 세션이다.
	RoleInteractive Role = "interactive"
	// RoleNonTTY는 자동화·CI·파이프라인 환경이다.
	RoleNonTTY Role = "non_tty"
)

// PermissionContext는 useCanUseTool 호출 시 전달되는 컨텍스트이다.
type PermissionContext struct {
	Role    Role
	Details map[string]any
}

// PermissionQueueOps는 YOLO classifier 및 자동 모드 기록 인터페이스이다.
// REQ-HK-012: SetYoloClassifierApproval, REQ-HK-009: RecordAutoModeDenial
type PermissionQueueOps interface {
	SetYoloClassifierApproval(toolPattern string)
	RecordAutoModeDenial(toolName string, reason string)
	LogPermissionDecision(result PermissionResult, toolName string)
}

// InteractiveHandler는 사용자 터미널 세션의 권한 처리 인터페이스이다.
// CLI-001이 구현한다.
type InteractiveHandler interface {
	PromptUser(ctx context.Context, toolName string, input map[string]any) (PermissionResult, error)
}

// CoordinatorHandler는 SUBAGENT-001 coordinator 모드의 권한 처리 인터페이스이다.
type CoordinatorHandler interface {
	RequestPermission(ctx context.Context, toolName string, input map[string]any) (PermissionResult, error)
}

// SwarmWorkerHandler는 SUBAGENT-001 swarm worker 모드의 권한 처리 인터페이스이다.
type SwarmWorkerHandler interface {
	BubbleUpPermission(ctx context.Context, toolName string, input map[string]any) (PermissionResult, error)
}

// SkillsFileChangedConsumer는 FileChanged 이벤트 후 호출되는 외부 consumer 타입이다.
// SKILLS-001이 등록하고 본 SPEC은 호출만 한다.
// D11 resolution
type SkillsFileChangedConsumer func(ctx context.Context, changed []string) []string

// WorkspaceRootResolver는 sessionID로 workspace root를 반환하는 인터페이스이다.
// SPEC-GOOSE-CORE-001이 구현한다. 본 SPEC은 consumer.
// D15 resolution / REQ-HK-021 b clause
type WorkspaceRootResolver interface {
	WorkspaceRoot(sessionID string) (string, error)
}

// PluginHookLoader는 plugin manifest에서 hook을 로드하는 인터페이스이다.
// PLUGIN-001이 구현한다.
type PluginHookLoader interface {
	// IsLoading은 플러그인 로드 중 여부를 반환한다.
	// REQ-HK-013: 로드 중에는 Register를 거부한다.
	IsLoading() bool
	// Load는 manifest에서 hook을 로드하여 registry에 등록한다.
	Load(manifest any, registry *HookRegistry) error
}

// 에러 sentinel 정의
var (
	// ErrRegistryLocked는 PluginHookLoader.IsLoading 중 Register 호출 시 반환된다.
	// REQ-HK-013 / AC-HK-014
	ErrRegistryLocked = errors.New("hook: registry is locked while plugin loader is loading")

	// ErrInvalidHookInput은 HookInput이 스키마 검증에 실패했을 때 반환된다.
	// REQ-HK-015 / AC-HK-016
	ErrInvalidHookInput = errors.New("hook: HookInput failed schema validation")

	// ErrHookPayloadTooLarge는 HookInput JSON이 4 MiB 초과 시 반환된다.
	// REQ-HK-022 / AC-HK-024
	ErrHookPayloadTooLarge = errors.New("hook: HookInput JSON exceeds 4 MiB limit")

	// ErrHookSessionUnresolved는 WorkspaceRoot resolver가 빈 경로/오류 반환 시 반환된다.
	// REQ-HK-021 b clause / AC-HK-022
	ErrHookSessionUnresolved = errors.New("hook: WorkspaceRoot resolver returned empty path or failed")

	// ErrInvalidConsumer는 nil consumer/resolver를 등록하려 할 때 반환된다.
	// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-008
	ErrInvalidConsumer = errors.New("hook: cannot register nil consumer or resolver")
)

// InteractiveOptsInternal은 InteractiveOpt 함수의 수신 타입이다.
// wire-up 코드에서 opts 상태를 읽기 위해 export된다.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-009
type InteractiveOptsInternal struct {
	ExplicitNoOp bool
}

// InteractiveOpt는 wireInteractiveHandler의 옵션 타입이다.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-009
type InteractiveOpt func(*InteractiveOptsInternal)

// WithExplicitNoOp은 nil handler 등록이 의도된 placeholder임을 표시한다.
// REQ-WIRE-008의 accidental-nil과 구분된다.
// SPEC-GOOSE-DAEMON-WIRE-001 REQ-WIRE-009
func WithExplicitNoOp() InteractiveOpt {
	return func(o *InteractiveOptsInternal) { o.ExplicitNoOp = true }
}

// maxPayloadBytes는 HookInput JSON 최대 크기 (4 MiB).
// REQ-HK-022 / D16: 정확히 4 MiB는 허용, 초과 시 에러.
const maxPayloadBytes = 4 * 1024 * 1024

// defaultShellTimeout은 shell command hook의 기본 타임아웃이다.
// REQ-HK-006 c, D17: cfg.Timeout <= 0 이면 이 값을 사용한다.
const defaultShellTimeout = 30 // 초

// defaultAsyncTimeout은 async hook의 기본 타임아웃이다.
// REQ-HK-010
const defaultAsyncTimeout = 60 // 초

// PreToolUseResult는 DispatchPreToolUse의 반환 타입이다.
type PreToolUseResult struct {
	Blocked            bool
	PermissionDecision *PermissionDecision
}

// PostToolUseResult는 DispatchPostToolUse의 반환 타입이다.
type PostToolUseResult struct {
	SuppressOutput       bool
	AdditionalContext    string
	UpdatedMCPToolOutput any
	Outputs              []HookJSONOutput
}

// SessionStartResult는 DispatchSessionStart의 반환 타입이다.
type SessionStartResult struct {
	InitialUserMessage string
	WatchPaths         []string
	AdditionalContext  string
}

// DispatchResult는 일반 Dispatch 함수의 반환 타입이다.
type DispatchResult struct {
	HandlerCount int
	Outcome      string // "ok" | "blocked" | "handler_error" | "timeout"
	Error        error
}
