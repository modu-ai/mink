// Package query는 QueryEngine과 관련 타입을 포함한다.
// SPEC-GOOSE-QUERY-001 S0 → S3+에서 구현된다.
package query

import (
	"context"
	"fmt"
	"sync"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query/loop"
)

// QueryEngine는 단일 대화 세션 = 단일 인스턴스의 agentic core 런타임이다.
// REQ-QUERY-001: 인스턴스 하나가 대화 세션 하나에 1:1 대응한다.
//
// @MX:ANCHOR: [AUTO] 모든 상위 레이어의 단일 진입점 (CLI, TRANSPORT, SUBAGENT)
// @MX:REASON: REQ-QUERY-001 - 대화 세션과 1:1 대응. fan_in >= 3 (CLI, test, future transport)
type QueryEngine struct {
	// cfg는 엔진 설정이다.
	cfg QueryEngineConfig
	// mu는 SubmitMessage 직렬화를 위한 뮤텍스이다.
	// REQ-QUERY-001: 동일 인스턴스에서 동시 SubmitMessage 호출 방지.
	mu sync.Mutex
	// state는 loop goroutine이 단독 소유하는 상태이다.
	// REQ-QUERY-015: 외부 goroutine이 직접 변경 금지.
	state loop.State
	// permInbox는 Ask permission 결정을 loop에 전달하는 채널이다.
	// REQ-QUERY-013: 외부에서 ResolvePermission을 통해 전송.
	//
	// @MX:WARN: [AUTO] 여러 goroutine에서 send, loop만 receive
	// @MX:REASON: REQ-QUERY-013 - Ask 분기 재개 단일 경로. buffering 정책 주의
	permInbox chan loop.PermissionDecision
}

// New는 QueryEngineConfig로 새 QueryEngine을 생성한다.
// REQ-QUERY-001: 유효성 검증 실패 시 에러 반환.
//
// @MX:TODO: [AUTO] S3 T3.1에서 유효성 검증 구현 필요.
func New(cfg QueryEngineConfig) (*QueryEngine, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid QueryEngineConfig: %w", err)
	}
	return &QueryEngine{
		cfg:       cfg,
		permInbox: make(chan loop.PermissionDecision, 1),
		state: loop.State{
			TaskBudgetRemaining: cfg.TaskBudget.Remaining,
		},
	}, nil
}

// SubmitMessage는 사용자 메시지를 제출하고 SDKMessage 스트림 채널을 반환한다.
// REQ-QUERY-005: 10ms 이내 반환 필수.
// REQ-QUERY-016: LLM 연결은 goroutine 내부에서 수행.
//
// @MX:WARN: [AUTO] goroutine spawn 지점 - State 단독 소유
// @MX:REASON: REQ-QUERY-015 - spawned goroutine이 State를 단독 소유. 외부 mutation 금지
// @MX:TODO: [AUTO] S3 T3.5에서 queryLoop 호출 구현 필요.
func (e *QueryEngine) SubmitMessage(ctx context.Context, prompt string) (<-chan message.SDKMessage, error) {
	// @MX:TODO: [AUTO] 최소 구현 필요. S3 T3.2에서 GREEN.
	panic("not implemented: SubmitMessage")
}

// ResolvePermission은 Ask permission 대기 중인 loop에 결정을 전달한다.
// REQ-QUERY-013: 외부 결정을 permInbox 채널을 통해 loop에 전달.
func (e *QueryEngine) ResolvePermission(toolUseID string, behavior int, reason string) {
	// @MX:TODO: [AUTO] S8 T8.2에서 구현 필요.
	panic("not implemented: ResolvePermission")
}

// validateConfig는 QueryEngineConfig 필수 필드를 검증한다.
func validateConfig(cfg QueryEngineConfig) error {
	if cfg.LLMCall == nil {
		return fmt.Errorf("LLMCall is required")
	}
	if cfg.CanUseTool == nil {
		return fmt.Errorf("CanUseTool is required")
	}
	if cfg.Executor == nil {
		return fmt.Errorf("Executor is required")
	}
	if cfg.Logger == nil {
		return fmt.Errorf("Logger is required")
	}
	return nil
}
