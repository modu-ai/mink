// Package query는 QueryEngine과 관련 타입을 포함한다.
// SPEC-GOOSE-QUERY-001 S0 T0.3
package query

import (
	"context"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/permissions"
	"github.com/modu-ai/mink/internal/query/loop"
	"go.uber.org/zap"
)

// TaskBudget는 QueryEngine의 token/cost 예산 설정이다.
// REQ-QUERY-011: remaining <= 0이면 budget_exceeded terminal.
type TaskBudget struct {
	// Total은 총 예산이다.
	Total int
	// Remaining은 남은 예산이다 (음수 가능).
	Remaining int
	// ToolResultCap은 tool result content의 최대 바이트 수이다 (0이면 무제한).
	// REQ-QUERY-007: 초과 시 요약 치환.
	ToolResultCap int
}

// TeammateIdentity는 teammate 모드에서 주입할 식별 정보이다.
// REQ-QUERY-020: system prompt header + SDKMessage meta 두 경로로 주입된다.
type TeammateIdentity struct {
	// AgentID는 에이전트 식별자이다.
	AgentID string
	// TeamName은 팀 이름이다.
	TeamName string
}

// ToolDefinition는 QueryEngine에 등록되는 도구 정의이다.
// spec.md §6.2 Tools manifest.
type ToolDefinition struct {
	// Name은 도구 이름이다.
	Name string
	// Description은 도구 설명이다.
	Description string
	// Scope는 도구 접근 범위이다 ("leader_only" | "worker_shareable" | "").
	// REQ-QUERY-012: CoordinatorMode=true 시 "leader_only" 도구는 LLM payload에서 제외.
	Scope string
	// InputSchema는 도구 입력 스키마이다 (JSON Schema 형식).
	InputSchema map[string]any
}

// Executor는 tool 실행 인터페이스이다.
// TOOLS-001에서 구현된다. 본 SPEC은 인터페이스 호출만 담당.
type Executor interface {
	// Run은 주어진 도구를 실행하고 결과를 반환한다.
	// toolUseID는 LLM 응답의 tool_use 블록 ID이다.
	Run(ctx context.Context, toolUseID, toolName string, input map[string]any) (string, error)
}

// Compactor는 대화 메시지 압축 인터페이스이다.
// CONTEXT-001에서 구현된다. 본 SPEC은 인터페이스 호출만 담당.
type Compactor interface {
	// ShouldCompact는 현재 상태에서 compaction이 필요한지 판단한다.
	ShouldCompact(state loop.State) bool
	// Compact는 현재 상태를 압축하고 새 상태와 경계 정보를 반환한다.
	Compact(state loop.State) (loop.State, CompactBoundary, error)
}

// CompactBoundary는 Compactor.Compact 반환 타입이다.
// SPEC-GOOSE-CONTEXT-001 §6.2
type CompactBoundary struct {
	// Turn은 compaction이 발생한 turn 번호이다.
	Turn int
	// Strategy는 선택된 compaction 전략이다 ("AutoCompact" | "ReactiveCompact" | "Snip").
	Strategy string
	// MessagesBefore는 compaction 전 메시지 수이다.
	MessagesBefore int
	// MessagesAfter는 compaction 후 메시지 수이다.
	MessagesAfter int
	// TokensBefore는 compaction 전 추정 token 수이다.
	TokensBefore int64
	// TokensAfter는 compaction 후 추정 token 수이다.
	TokensAfter int64
	// TaskBudgetPreserved는 compaction 전후 TaskBudgetRemaining 값이다 (불변 검증용).
	// REQ-CTX-010: compaction 자체는 task budget을 소비하지 않는다.
	TaskBudgetPreserved int64
	// DroppedThinkingCount는 보존된 redacted_thinking 블록 수이다.
	DroppedThinkingCount int
}

// MessageHook는 LLM 응답 샘플링 후 호출되는 훅 함수 타입이다.
// REQ-QUERY-018: PostSamplingHooks FIFO chain.
type MessageHook func(ctx context.Context, msg message.Message) (message.Message, error)

// FailureHook는 loop 비복구 오류 시 호출되는 훅 함수 타입이다.
// REQ-QUERY-014: StopFailureHooks.
type FailureHook func(ctx context.Context, err error)

// QueryEngineConfig는 QueryEngine 생성자에 전달되는 설정이다.
// spec.md §6.2 QueryEngineConfig 필드 정의.
type QueryEngineConfig struct {
	// LLMCall은 LLM API 호출 함수이다. 필수.
	// ADAPTER-001에서 구현되는 LLMCallFunc를 주입받는다.
	LLMCall LLMCallFunc
	// Tools는 등록된 도구 목록이다. 필수 (빈 슬라이스 허용).
	Tools []ToolDefinition
	// CanUseTool는 tool 실행 권한 게이트이다. 필수.
	CanUseTool permissions.CanUseTool
	// Executor는 tool 실행 엔진이다. 필수.
	Executor Executor
	// Logger는 구조화 로거이다. 필수.
	Logger *zap.Logger
	// TaskBudget는 token/cost 예산 설정이다. 선택적.
	TaskBudget TaskBudget
	// MaxTurns는 최대 turn 수이다. 0이면 즉시 max_turns terminal.
	MaxTurns int
	// Compactor는 메시지 압축 인터페이스이다. 선택적 (nil이면 compaction 비활성).
	Compactor Compactor
	// PostSamplingHooks는 LLM 응답 후 FIFO 순으로 호출되는 훅 목록이다.
	// REQ-QUERY-018: Optional.
	PostSamplingHooks []MessageHook
	// StopFailureHooks는 loop 비복구 오류 시 호출되는 훅 목록이다.
	StopFailureHooks []FailureHook
	// FallbackModels는 primary 모델 실패 시 순서대로 시도할 모델 목록이다.
	// REQ-QUERY-019: Optional.
	FallbackModels []string
	// CoordinatorMode는 coordinator 모드 활성화 여부이다.
	// REQ-QUERY-012: true이면 scope:"leader_only" 도구를 LLM payload에서 제외.
	CoordinatorMode bool
	// TeammateIdentity는 teammate 식별 정보이다. 선택적.
	// REQ-QUERY-020: nil이면 주입 없음.
	TeammateIdentity *TeammateIdentity
}
