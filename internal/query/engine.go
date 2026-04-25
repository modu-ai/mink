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
	// stateMu는 e.state 읽기/쓰기를 보호하는 전용 뮤텍스이다.
	// S5: loop goroutine의 OnComplete와 SubmitMessage 간 race 방지.
	// mu와 별도로 유지하여 loop goroutine이 mu 없이 state를 갱신할 수 있도록 한다.
	stateMu sync.Mutex
	// state는 SubmitMessage 호출 간 누적되는 대화 상태이다.
	// REQ-QUERY-015: loop goroutine만 직접 변경. 외부는 stateMu로 보호.
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
// @MX:ANCHOR: [AUTO] 모든 상위 레이어의 단일 스트리밍 진입점
// @MX:REASON: REQ-QUERY-001/005 - 10ms 마감 + 세션 1:1 대응. fan_in >= 3
// @MX:WARN: [AUTO] goroutine spawn 지점 - State 단독 소유
// @MX:REASON: REQ-QUERY-015 - spawned goroutine이 State를 단독 소유. 외부 mutation 금지
func (e *QueryEngine) SubmitMessage(ctx context.Context, prompt string) (<-chan message.SDKMessage, error) {
	// REQ-QUERY-004: SubmitMessage 호출을 직렬화하여 동시 실행 방지.
	e.mu.Lock()
	defer e.mu.Unlock()

	// unbuffered 채널: REQ-QUERY-002 - 송신 불가(receive-only) 계약
	out := make(chan message.SDKMessage)

	// S5: user message를 state에 포함시킨 스냅샷을 loop에 전달한다.
	// REQ-QUERY-004: messages 누적 보존. loop 완료 후 OnComplete로 갱신됨.
	userMsg := message.Message{
		Role: "user",
		Content: []message.ContentBlock{
			{Type: "text", Text: prompt},
		},
	}
	e.stateMu.Lock()
	currentState := e.state
	e.stateMu.Unlock()
	currentState.Messages = append(append([]message.Message(nil), currentState.Messages...), userMsg)

	// LLM 호출 클로저: messages + tools를 바인딩하여 loop에 전달한다.
	// REQ-QUERY-016: LLM 연결(dial) 비용은 goroutine 내부에서 처리한다.
	// currentState.Messages는 이미 user message를 포함한 스냅샷이다.
	callLLM := e.buildLLMStreamFuncFromMsgs(currentState.Messages)

	cfg := loop.LoopConfig{
		Out:          out,
		InitialState: currentState,
		Prompt:       prompt,
		MaxTurns:     e.cfg.MaxTurns,
		PermInbox:    e.permInbox,
		CallLLM:      callLLM,
		// S4: tool roundtrip 의존성 주입
		CallLLMFactory: e.buildLLMStreamFuncFactory(),
		CanUseTool:     e.cfg.CanUseTool,
		Execute:        e.cfg.Executor.Run,
		ToolResultCap:  e.cfg.TaskBudget.ToolResultCap,
		// S5: loop 종료 후 최종 State를 engine.state에 반영한다.
		// REQ-QUERY-004: SubmitMessage 호출 간 messages/turnCount/budget 누적 보존.
		//
		// @MX:NOTE: [AUTO] OnComplete는 loop goroutine의 defer 체인에서 호출된다.
		// stateMu로 e.state 쓰기를 보호하여 SubmitMessage의 e.state 읽기와의 경합을 방지한다.
		OnComplete: func(finalState loop.State) {
			e.stateMu.Lock()
			e.state = finalState
			e.stateMu.Unlock()
		},
	}

	// loop.Run은 goroutine을 spawn하고 즉시 반환한다.
	// out 채널 close 책임은 queryLoop 단독.
	loop.Run(ctx, cfg)

	return out, nil
}

// buildLLMStreamFuncFromMsgs는 주어진 messages 슬라이스를 사용하는 LLMStreamFunc를 생성한다.
// REQ-QUERY-016: LLM 연결 비용이 이 클로저 호출 시점(goroutine 내부)에서 발생한다.
// S5: currentState.Messages (user message 포함)를 직접 전달하여 e.state 의존을 제거한다.
func (e *QueryEngine) buildLLMStreamFuncFromMsgs(msgs []message.Message) loop.LLMStreamFunc {
	llmCall := e.cfg.LLMCall

	return func(ctx context.Context) (<-chan message.StreamEvent, error) {
		req := LLMCallReq{
			Messages: msgs,
		}
		return llmCall(ctx, req)
	}
}

// buildLLMStreamFuncFactory는 messages를 갱신하여 새 LLMStreamFunc를 생성하는 factory를 반환한다.
// S4 after_tool_results continue site에서 tool_result를 포함한 다음 LLM 호출 클로저를 만들 때 사용된다.
func (e *QueryEngine) buildLLMStreamFuncFactory() func(msgs []message.Message) loop.LLMStreamFunc {
	llmCall := e.cfg.LLMCall
	return func(msgs []message.Message) loop.LLMStreamFunc {
		// 전달받은 messages 스냅샷을 사용하는 LLMStreamFunc를 반환한다.
		return func(ctx context.Context) (<-chan message.StreamEvent, error) {
			req := LLMCallReq{
				Messages: msgs,
			}
			return llmCall(ctx, req)
		}
	}
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
		return fmt.Errorf("field LLMCall is required")
	}
	if cfg.CanUseTool == nil {
		return fmt.Errorf("field CanUseTool is required")
	}
	if cfg.Executor == nil {
		return fmt.Errorf("field Executor is required")
	}
	if cfg.Logger == nil {
		return fmt.Errorf("field Logger is required")
	}
	return nil
}
