// Package loop는 QueryEngine의 queryLoop goroutine과 상태 머신을 포함한다.
// SPEC-GOOSE-QUERY-001 S0 T3.5
package loop

import "github.com/modu-ai/goose/internal/message"

// State는 queryLoop가 단독으로 소유하는 가변 상태이다.
// REQ-QUERY-015: 외부 goroutine이 직접 변경해서는 안 된다.
type State struct {
	// Messages는 누적된 대화 메시지 배열이다.
	// REQ-QUERY-004: SubmitMessage 호출 간 공유된다.
	Messages []message.Message
	// TurnCount는 완료된 turn 수이다.
	// REQ-QUERY-011: maxTurns 도달 시 terminal 발생.
	TurnCount int
	// MaxOutputTokensRecoveryCount는 max_output_tokens 재시도 횟수이다.
	// REQ-QUERY-008: ≤3이면 재시도, >3이면 terminal.
	MaxOutputTokensRecoveryCount int
	// TaskBudgetRemaining는 남은 task budget 수이다.
	// REQ-QUERY-011: ≤0이면 budget_exceeded terminal.
	TaskBudgetRemaining int
}

// Continue는 queryLoop이 다음 iteration을 계속할 때의 신호 타입이다.
// REQ-QUERY-003: 오직 3개의 continue site에서만 생성된다.
type Continue struct {
	// Reason은 continue 발생 이유이다.
	// "after_compact" | "after_retry" | "after_tool_results" 3종만 허용.
	Reason string
	// NewState는 continue site에서 전환된 새 상태이다.
	NewState State
}

// continue site reason 상수 - REQ-QUERY-003 "오직 이 3곳" 계약.
const (
	// ReasonAfterCompact는 compaction 완료 후 continue site이다.
	ReasonAfterCompact = "after_compact"
	// ReasonAfterRetry는 max_output_tokens 재시도 후 continue site이다.
	ReasonAfterRetry = "after_retry"
	// ReasonAfterToolResults는 tool 실행 결과 처리 후 continue site이다.
	ReasonAfterToolResults = "after_tool_results"
)

// Terminal는 queryLoop 종료 상태이다.
// loop는 Terminal을 생성하고 출력 채널에 SDKMsgTerminal을 yield한 뒤 반환한다.
type Terminal struct {
	// Success는 loop가 성공적으로 완료되었는지 여부이다.
	// REQ-QUERY-011: max_turns 도달은 success=true.
	Success bool
	// Error는 종료 이유이다.
	// "" | "max_turns" | "budget_exceeded" | "aborted" | "max_output_tokens_exhausted" | provider error
	Error string
}
