// Package loop는 QueryEngine의 queryLoop goroutine과 상태 머신을 포함한다.
// SPEC-GOOSE-QUERY-001 S3에서 최소 구현 (tool 없는 단일 턴 시나리오).
package loop

import (
	"context"

	"github.com/modu-ai/goose/internal/message"
)

// LLMStreamFunc는 queryLoop이 LLM API를 호출하는 함수 타입이다.
// engine이 LLMCallFunc를 감싸서 주입한다.
// S3 범위: tool 없는 단일 호출만 처리한다.
type LLMStreamFunc func(ctx context.Context) (<-chan message.StreamEvent, error)

// LoopConfig는 queryLoop 실행에 필요한 의존성 묶음이다.
// QueryEngine에서 SubmitMessage 호출 시 생성하여 queryLoop에 전달한다.
type LoopConfig struct {
	// Out은 SDKMessage를 전송하는 출력 채널이다. loop만 close한다.
	//
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
	// CallLLM은 LLM 스트림을 시작하는 함수이다.
	// engine이 LLMCallFunc를 감싸서 필요한 messages/tools를 바인딩한 클로저로 전달한다.
	CallLLM LLMStreamFunc
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
func queryLoop(ctx context.Context, cfg LoopConfig) {
	defer close(cfg.Out)

	state := cfg.InitialState

	// 1. user_ack yield: 사용자 요청 즉시 확인
	if !send(ctx, cfg.Out, message.SDKMessage{
		Type:    message.SDKMsgUserAck,
		Payload: message.PayloadUserAck{Prompt: cfg.Prompt},
	}) {
		return
	}

	// S3 T3.5 범위: tool 없는 단일 assistant 턴만 처리한다.
	// S4+ (tool roundtrip, permission, compaction 등)는 후속 단계에서 구현한다.

	// 2. turn 카운트 증가 + stream_request_start yield
	state.TurnCount++
	if !send(ctx, cfg.Out, message.SDKMessage{
		Type:    message.SDKMsgStreamRequestStart,
		Payload: message.PayloadStreamRequestStart{Turn: state.TurnCount},
	}) {
		return
	}

	// 3. LLM 스트림 호출
	streamCh, err := cfg.CallLLM(ctx)
	if err != nil {
		_ = send(ctx, cfg.Out, message.SDKMessage{
			Type:    message.SDKMsgTerminal,
			Payload: message.PayloadTerminal{Success: false, Error: err.Error()},
		})
		return
	}

	// 4. 스트림 이벤트 처리: delta 누적 + yield
	var textBuf string
	for ev := range streamCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		switch ev.Type {
		case message.TypeTextDelta:
			textBuf += ev.Delta
			if !send(ctx, cfg.Out, message.SDKMessage{
				Type:    message.SDKMsgStreamEvent,
				Payload: message.PayloadStreamEvent{Event: ev},
			}) {
				return
			}
		case message.TypeMessageStop:
			// message_stop은 스트리밍 종료 신호. 별도 yield 없이 다음 단계로 진행.
		default:
			// 기타 이벤트는 그대로 전달
			if !send(ctx, cfg.Out, message.SDKMessage{
				Type:    message.SDKMsgStreamEvent,
				Payload: message.PayloadStreamEvent{Event: ev},
			}) {
				return
			}
		}
	}

	// 5. 완성된 assistant message 조립 후 state에 누적
	assistantMsg := message.Message{
		Role: "assistant",
		Content: []message.ContentBlock{
			{Type: "text", Text: textBuf},
		},
	}
	state.Messages = append(state.Messages, assistantMsg)

	// 6. assistant message yield
	if !send(ctx, cfg.Out, message.SDKMessage{
		Type:    message.SDKMsgMessage,
		Payload: message.PayloadMessage{Msg: assistantMsg},
	}) {
		return
	}

	// 7. terminal{success:true} yield
	// after_assistant_terminal 경로 (tool 없는 시나리오, S3 범위)
	_ = send(ctx, cfg.Out, message.SDKMessage{
		Type:    message.SDKMsgTerminal,
		Payload: message.PayloadTerminal{Success: true},
	})
}

// send는 채널로 SDKMessage를 전송한다.
// ctx가 취소되면 false를 반환한다.
//
// @MX:WARN: [AUTO] out 채널 전송 시 ctx.Done() 경합 처리
// @MX:REASON: REQ-QUERY-010 - abort 시 500ms 내 정상 종료
func send(ctx context.Context, out chan<- message.SDKMessage, msg message.SDKMessage) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- msg:
		return true
	}
}

// Run은 queryLoop를 goroutine으로 실행한다.
// SubmitMessage에서 호출된다. out 채널 close 책임은 queryLoop 단독.
//
// @MX:ANCHOR: [AUTO] goroutine spawn 공개 진입점
// @MX:REASON: REQ-QUERY-002 - SubmitMessage가 이 함수를 통해서만 loop를 시작한다
func Run(ctx context.Context, cfg LoopConfig) {
	go queryLoop(ctx, cfg)
}
