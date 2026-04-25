// Package loop는 QueryEngine의 queryLoop goroutine과 상태 머신을 포함한다.
// SPEC-GOOSE-QUERY-001 S3: 최소 구현 (tool 없는 단일 턴 시나리오).
// SPEC-GOOSE-QUERY-001 S4: tool roundtrip, permission Allow/Deny 분기, tool_result budget 치환.
//
// 상태 머신 경로 요약:
//   - 경로 A: tool 없는 assistant turn → terminal{success:true}
//   - 경로 B: tool_use → permission(Allow/Deny) → tool_result → after_tool_results → 다음 turn
package loop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/permissions"
)

// LLMStreamFunc는 queryLoop이 LLM API를 호출하는 함수 타입이다.
// engine이 LLMCallFunc를 감싸서 주입한다.
type LLMStreamFunc func(ctx context.Context) (<-chan message.StreamEvent, error)

// ExecutorFunc는 tool 실행 함수 타입이다.
// engine이 Executor.Run을 감싸서 주입한다.
type ExecutorFunc func(ctx context.Context, toolUseID, toolName string, input map[string]any) (string, error)

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
	// CallLLMFactory는 갱신된 messages 슬라이스로 새 LLMStreamFunc를 생성하는 factory이다.
	// S4 after_tool_results continue site에서 tool_result를 포함한 다음 LLM 호출 클로저를 만든다.
	// nil이면 CallLLM을 재사용한다.
	CallLLMFactory func(msgs []message.Message) LLMStreamFunc
	// CanUseTool은 tool 실행 전 권한을 확인하는 gate이다.
	// S4+에서 permission 분기 처리에 사용된다.
	CanUseTool permissions.CanUseTool
	// Execute는 tool 실행 함수이다.
	// S4+에서 Allow 분기에서 호출된다.
	Execute ExecutorFunc
	// ToolResultCap은 tool result content의 최대 바이트 수이다 (0이면 무제한).
	// REQ-QUERY-007: 초과 시 요약 치환.
	ToolResultCap int
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

// toolUseBlock은 LLM 응답 스트림에서 누적된 단일 tool_use 블록이다.
type toolUseBlock struct {
	toolUseID string
	toolName  string
	inputJSON string // content_block_start Delta에서 이름, input_json_delta에서 누적
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

	// S4: tool roundtrip을 지원하는 main loop.
	// tool 없는 경우 단일 턴으로 종료된다.
	for {
		// turn 카운트 증가 + stream_request_start yield
		state.TurnCount++
		if !send(ctx, cfg.Out, message.SDKMessage{
			Type:    message.SDKMsgStreamRequestStart,
			Payload: message.PayloadStreamRequestStart{Turn: state.TurnCount},
		}) {
			return
		}

		// LLM 스트림 호출
		streamCh, err := cfg.CallLLM(ctx)
		if err != nil {
			_ = send(ctx, cfg.Out, message.SDKMessage{
				Type:    message.SDKMsgTerminal,
				Payload: message.PayloadTerminal{Success: false, Error: err.Error()},
			})
			return
		}

		// 스트림 이벤트 처리: delta 누적 + tool_use 블록 파싱
		var textBuf string
		var toolBlocks []toolUseBlock
		var curTool *toolUseBlock

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

			case message.TypeContentBlockStart:
				// tool_use 블록 시작: ToolUseID + Delta(=toolName)
				if ev.BlockType == "tool_use" {
					curTool = &toolUseBlock{
						toolUseID: ev.ToolUseID,
						toolName:  ev.Delta,
					}
				}
				// stream_event로 전달
				if !send(ctx, cfg.Out, message.SDKMessage{
					Type:    message.SDKMsgStreamEvent,
					Payload: message.PayloadStreamEvent{Event: ev},
				}) {
					return
				}

			case message.TypeInputJSONDelta:
				// tool input JSON 누적
				if curTool != nil {
					curTool.inputJSON += ev.Delta
				}
				if !send(ctx, cfg.Out, message.SDKMessage{
					Type:    message.SDKMsgStreamEvent,
					Payload: message.PayloadStreamEvent{Event: ev},
				}) {
					return
				}

			case message.TypeContentBlockStop:
				// 현재 tool_use 블록 완료
				if curTool != nil {
					toolBlocks = append(toolBlocks, *curTool)
					curTool = nil
				}
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

		// assistant message 조립
		assistantBlocks := buildAssistantBlocks(textBuf, toolBlocks)
		assistantMsg := message.Message{
			Role:    "assistant",
			Content: assistantBlocks,
		}
		state.Messages = append(state.Messages, assistantMsg)

		// assistant message yield
		if !send(ctx, cfg.Out, message.SDKMessage{
			Type:    message.SDKMsgMessage,
			Payload: message.PayloadMessage{Msg: assistantMsg},
		}) {
			return
		}

		// tool_use 블록이 없으면 terminal{success:true} 후 종료
		if len(toolBlocks) == 0 {
			_ = send(ctx, cfg.Out, message.SDKMessage{
				Type:    message.SDKMsgTerminal,
				Payload: message.PayloadTerminal{Success: true},
			})
			return
		}

		// --- after_tool_results continue site ---
		// tool_use 블록이 있으면 permission 체크 → 실행 → tool_result 조립 → 다음 turn
		toolResultBlocks, ok := processToolUseBlocks(ctx, cfg, state.TurnCount, toolBlocks)
		if !ok {
			// ctx 취소 시
			return
		}

		// tool_result를 user message로 state에 추가 (다음 LLM 호출에 포함)
		toolResultMsg := message.Message{
			Role:    "user",
			Content: toolResultBlocks,
		}
		state.Messages = append(state.Messages, toolResultMsg)

		// after_tool_results: CallLLM 클로저를 tool_result 포함한 버전으로 갱신
		cfg.CallLLM = buildLLMFuncWithMessages(cfg, state.Messages)

		// 다음 turn으로 계속 (continue site: after_tool_results)
	}
}

// buildAssistantBlocks는 텍스트 버퍼와 tool_use 블록에서 ContentBlock 슬라이스를 생성한다.
func buildAssistantBlocks(textBuf string, toolBlocks []toolUseBlock) []message.ContentBlock {
	var blocks []message.ContentBlock
	if textBuf != "" {
		blocks = append(blocks, message.ContentBlock{Type: "text", Text: textBuf})
	}
	for _, tb := range toolBlocks {
		blocks = append(blocks, message.ContentBlock{
			Type:      "tool_use",
			ToolUseID: tb.toolUseID,
			Text:      tb.toolName,
			// inputJSON은 ToolResultJSON 필드 재사용 (tool_use input으로 사용)
			ToolResultJSON: tb.inputJSON,
		})
	}
	return blocks
}

// processToolUseBlocks는 tool_use 블록 슬라이스에 대해 순차적으로
// permission 체크 → Allow 시 실행 / Deny 시 에러 합성을 처리한다.
// 반환: (tool_result ContentBlock 슬라이스, ctx 취소 여부)
//
// @MX:NOTE: [AUTO] after_tool_results continue site의 핵심 로직.
// @MX:SPEC: REQ-QUERY-006, REQ-QUERY-003
func processToolUseBlocks(ctx context.Context, cfg LoopConfig, turn int, toolBlocks []toolUseBlock) ([]message.ContentBlock, bool) {
	var resultBlocks []message.ContentBlock

	for _, tb := range toolBlocks {
		// permission check
		tpc := permissions.ToolPermissionContext{
			ToolUseID: tb.toolUseID,
			ToolName:  tb.toolName,
			Input:     parseInputJSON(tb.inputJSON),
			Turn:      turn,
		}

		decision := cfg.CanUseTool.Check(ctx, tpc)

		switch decision.Behavior {
		case permissions.Allow:
			// permission_check{allow} yield
			if !send(ctx, cfg.Out, message.SDKMessage{
				Type: message.SDKMsgPermissionCheck,
				Payload: message.PayloadPermissionCheck{
					ToolUseID: tb.toolUseID,
					Behavior:  "allow",
				},
			}) {
				return nil, false
			}

			// Executor.Run 호출
			result, execErr := cfg.Execute(ctx, tb.toolUseID, tb.toolName, tpc.Input)
			if execErr != nil {
				// 실행 에러: is_error=true tool_result로 합성
				errResult := fmt.Sprintf(`{"error":%q}`, execErr.Error())
				resultBlocks = append(resultBlocks, message.ContentBlock{
					Type:           "tool_result",
					ToolUseID:      tb.toolUseID,
					ToolResultJSON: errResult,
				})
				continue
			}

			// tool_result budget 검사 및 치환
			toolResultJSON := applyToolResultCap(tb.toolUseID, result, cfg.ToolResultCap)
			resultBlocks = append(resultBlocks, message.ContentBlock{
				Type:           "tool_result",
				ToolUseID:      tb.toolUseID,
				ToolResultJSON: toolResultJSON,
			})

		case permissions.Deny:
			// permission_check{deny} yield
			if !send(ctx, cfg.Out, message.SDKMessage{
				Type: message.SDKMsgPermissionCheck,
				Payload: message.PayloadPermissionCheck{
					ToolUseID: tb.toolUseID,
					Behavior:  "deny",
					Reason:    decision.Reason,
				},
			}) {
				return nil, false
			}

			// Executor 호출 없이 error result 합성
			denied := permissions.SynthesizeDeniedResult(tb.toolUseID, decision)
			deniedJSON, _ := json.Marshal(map[string]any{
				"denied": true,
				"reason": denied.Content,
			})
			resultBlocks = append(resultBlocks, message.ContentBlock{
				Type:           "tool_result",
				ToolUseID:      tb.toolUseID,
				ToolResultJSON: string(deniedJSON),
			})

		case permissions.Ask:
			// S6에서 구현 예정 (Ask 분기는 S4 범위 외)
			// 현재는 Deny와 동일하게 처리
			if !send(ctx, cfg.Out, message.SDKMessage{
				Type: message.SDKMsgPermissionCheck,
				Payload: message.PayloadPermissionCheck{
					ToolUseID: tb.toolUseID,
					Behavior:  "deny",
					Reason:    "ask_not_supported_in_s4",
				},
			}) {
				return nil, false
			}
			denied := permissions.SynthesizeDeniedResult(tb.toolUseID, decision)
			deniedJSON, _ := json.Marshal(map[string]any{
				"denied": true,
				"reason": denied.Content,
			})
			resultBlocks = append(resultBlocks, message.ContentBlock{
				Type:           "tool_result",
				ToolUseID:      tb.toolUseID,
				ToolResultJSON: string(deniedJSON),
			})
		}
	}

	return resultBlocks, true
}

// applyToolResultCap은 tool 결과가 cap을 초과하면 요약 치환한다.
// cap이 0이면 원본 그대로 반환한다.
// REQ-QUERY-007: 초과 시 {tool_use_id, truncated:true, bytes_original, bytes_kept} 치환.
func applyToolResultCap(toolUseID, result string, cap int) string {
	if cap <= 0 || len(result) <= cap {
		return result
	}

	// cap 초과: 요약 치환
	kept := result[:cap]
	summary := message.FormatToolUseSummary(toolUseID, nil, true, len(result), len(kept))
	summaryJSON, _ := json.Marshal(map[string]any{
		"truncated":      summary.Truncated,
		"bytes_original": summary.BytesOriginal,
		"bytes_kept":     summary.BytesKept,
		"preview":        kept,
	})
	return string(summaryJSON)
}

// buildLLMFuncWithMessages는 갱신된 messages 슬라이스를 사용하는 LLMStreamFunc를 생성한다.
// after_tool_results continue site에서 tool_result를 포함한 다음 LLM 호출 클로저를 만든다.
//
// @MX:NOTE: [AUTO] after_tool_results continue site에서 다음 LLM 호출 클로저 재생성.
// 내부 구현: cfg.CallLLM을 직접 사용할 수 없어서 engine에서 주입받은 factory를 활용한다.
// S4: CallLLMFactory를 통해 messages를 갱신한 클로저를 생성한다.
func buildLLMFuncWithMessages(cfg LoopConfig, msgs []message.Message) LLMStreamFunc {
	// CallLLMFactory가 있으면 사용, 없으면 기존 CallLLM 재사용 (engine이 messages를 관리하는 경우)
	if cfg.CallLLMFactory != nil {
		return cfg.CallLLMFactory(msgs)
	}
	// factory가 없으면 기존 함수 재사용 (engine이 state를 업데이트한 뒤 다음 호출 시 반영)
	return cfg.CallLLM
}

// parseInputJSON은 JSON 문자열을 map[string]any로 파싱한다.
// 파싱 실패 시 빈 맵을 반환한다.
func parseInputJSON(s string) map[string]any {
	if s == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return map[string]any{}
	}
	return m
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
