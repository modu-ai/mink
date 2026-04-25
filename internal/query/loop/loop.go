// Package loop는 QueryEngine의 queryLoop goroutine과 상태 머신을 포함한다.
// SPEC-GOOSE-QUERY-001 S0 → S4+에서 구현된다.
package loop

import (
	"context"

	"github.com/modu-ai/goose/internal/message"
)

// LoopConfig는 queryLoop 실행에 필요한 의존성 묶음이다.
// QueryEngine에서 SubmitMessage 호출 시 생성하여 queryLoop에 전달한다.
type LoopConfig struct {
	// Out은 SDKMessage를 전송하는 출력 채널이다. loop만 close한다.
	// @MX:WARN: [AUTO] loop 단독 close 소유자. 이중 close 패닉 방지.
	// @MX:REASON: REQ-QUERY-002/010 - close 단일 소유자 계약
	Out chan<- message.SDKMessage
	// InitialState는 이번 turn 시작 시 전달된 초기 상태이다.
	InitialState State
	// Prompt는 이번 turn의 사용자 메시지이다.
	Prompt string
	// MaxTurns는 최대 turn 수이다.
	MaxTurns int
	// PermInbox는 Ask permission 결정을 수신하는 채널이다.
	PermInbox <-chan PermissionDecision
}

// PermissionDecision은 외부에서 Ask 권한 결정을 전달하는 타입이다.
// REQ-QUERY-013: ResolvePermission API를 통해 전달된다.
type PermissionDecision struct {
	// ToolUseID는 결정 대상 tool_use 블록 ID이다.
	ToolUseID string
	// Behavior는 결정 결과이다 (Allow | Deny).
	// Ask로 전달하는 것은 허용되지 않는다.
	Behavior int // permissions.PermissionBehavior
	// Reason은 Deny 시 이유이다.
	Reason string
}

// queryLoop는 agentic core의 상태 머신 본체이다.
// SubmitMessage에서 goroutine으로 spawn되며, out 채널 close 책임을 단독으로 진다.
//
// @MX:ANCHOR: [AUTO] agentic core 상태 머신 본체 - continue site 재할당 불변식의 중심
// @MX:REASON: REQ-QUERY-003 - 오직 3개의 continue site(after_compact/after_retry/after_tool_results)에서만 State 변경
// @MX:WARN: [AUTO] goroutine spawn 지점 - State 단독 소유자
// @MX:REASON: REQ-QUERY-015 - state ownership은 loop goroutine 단일. 외부 mutation 금지
// @MX:TODO: [AUTO] S3 GREEN에서 구현. 현재 stub.
func queryLoop(ctx context.Context, cfg LoopConfig) {
	// @MX:TODO: [AUTO] 최소 구현 필요. S3 T3.5에서 GREEN.
	panic("not implemented: queryLoop")
}
