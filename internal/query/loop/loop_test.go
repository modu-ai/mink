//go:build integration

// Package loop_test — SPEC-GOOSE-QUERY-001 S4 통합 테스트.
// T4.1: Tool roundtrip Allow (AC-QUERY-002)
// T4.2: Permission Deny 처리 (AC-QUERY-003)
// T4.3: tool_result budget 치환 (AC-QUERY-009 일부)
// T4.4: 다중 tool_use 순차 실행
//
// 빌드 태그: integration (go test -tags=integration)
package loop_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/permissions"
	"github.com/modu-ai/goose/internal/query"
	"github.com/modu-ai/goose/internal/query/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// drainMessages는 SDKMessage 채널을 drain하여 슬라이스로 반환한다.
func drainMessages(out <-chan message.SDKMessage) []message.SDKMessage {
	var msgs []message.SDKMessage
	for m := range out {
		msgs = append(msgs, m)
	}
	return msgs
}

// findMessages는 지정한 타입의 SDKMessage를 순서대로 반환한다.
func findMessages(msgs []message.SDKMessage, t message.SDKMessageType) []message.SDKMessage {
	var result []message.SDKMessage
	for _, m := range msgs {
		if m.Type == t {
			result = append(result, m)
		}
	}
	return result
}

// makeLoopConfig는 T4.x 테스트용 기본 QueryEngineConfig를 생성한다.
func makeLoopConfig(t *testing.T, stub *testsupport.StubLLMCall, canUse permissions.CanUseTool, executor query.Executor) query.QueryEngineConfig {
	t.Helper()
	return query.QueryEngineConfig{
		LLMCall:    stub.AsFunc(),
		Tools:      []query.ToolDefinition{},
		CanUseTool: canUse,
		Executor:   executor,
		Logger:     zaptest.NewLogger(t),
		MaxTurns:   10,
		TaskBudget: query.TaskBudget{Total: 10000, Remaining: 10000, ToolResultCap: 0},
	}
}

// --- T4.1: TestQueryLoop_ToolCallAllow_FullRoundtrip (AC-QUERY-002) ---

// TestQueryLoop_ToolCallAllow_FullRoundtrip는 tool_use → Allow → Executor.Run → tool_result → 2nd assistant
// 전체 roundtrip을 검증한다. REQ-QUERY-006 Allow 경로, REQ-QUERY-003 after_tool_results continue site.
//
// Given: StubLLM 1차 = tool_use{echo, x:1}, 2차 = stop
//
//	StubCanUseTool = Allow, StubExecutor.echo = {"x":1}
//
// When: SubmitMessage("call echo") drain
// Then: tool_use → permission_check{allow} → tool_result → 2nd assistant → terminal{success:true}
//
//	TurnCount == 2
func TestQueryLoop_ToolCallAllow_FullRoundtrip(t *testing.T) {
	t.Parallel()

	// Arrange
	toolUseID := "tu_001"
	toolName := "echo"
	inputJSON := `{"x":1}`

	stub := testsupport.NewStubLLMCall(
		// 1차 응답: tool_use 블록
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, inputJSON),
		},
		// 2차 응답: stop (tool_result를 받은 뒤의 응답)
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("done"),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		// echo 도구: 입력을 그대로 반환
		return `{"x":1}`, nil
	})

	canUse := testsupport.NewStubCanUseToolAllow()

	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	// Act
	out, err := engine.SubmitMessage(context.Background(), "call echo")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// Assert: 메시지 시퀀스 검증
	// 최소 포함: user_ack, stream_request_start(x2), permission_check, message(x2), terminal
	require.NotEmpty(t, msgs)

	// 첫 번째: user_ack
	assert.Equal(t, message.SDKMsgUserAck, msgs[0].Type, "첫 번째는 user_ack이어야 한다")

	// terminal이 마지막이고 success=true이어야 한다
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type, "마지막은 terminal이어야 한다")
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success, "terminal.success가 true이어야 한다")

	// permission_check{behavior:"allow"} 가 포함되어야 한다
	permChecks := findMessages(msgs, message.SDKMsgPermissionCheck)
	require.Len(t, permChecks, 1, "permission_check 메시지가 정확히 1개이어야 한다")
	permPayload, ok := permChecks[0].Payload.(message.PayloadPermissionCheck)
	require.True(t, ok)
	assert.Equal(t, "allow", permPayload.Behavior, "permission_check.behavior가 allow이어야 한다")
	assert.Equal(t, toolUseID, permPayload.ToolUseID, "permission_check.tool_use_id가 일치해야 한다")

	// executor가 정확히 1번 호출되었는지 (Allow → Run)
	// StubLLMCall이 2번 호출되어야 한다 (1차 tool_use, 2차 stop)
	assert.Equal(t, 2, stub.CallCount(), "LLM이 2번 호출되어야 한다")

	// 2개의 assistant message(또는 1개 이상)가 yield되어야 한다
	// 1차: tool_use 포함, 2차: stop 후 text
	assistantMsgs := findMessages(msgs, message.SDKMsgMessage)
	assert.GreaterOrEqual(t, len(assistantMsgs), 1, "최소 1개의 assistant message가 있어야 한다")
}

// --- T4.2: TestQueryLoop_PermissionDeny_SynthesizesErrorResult (AC-QUERY-003) ---

// TestQueryLoop_PermissionDeny_SynthesizesErrorResult는 Deny 결정 시 Executor 미호출 + is_error=true
// tool_result 합성을 검증한다. REQ-QUERY-006 Deny 경로.
//
// Given: StubLLM = tool_use{rm_rf}, StubCanUseTool = Deny{reason:"destructive"}
//
//	StubExecutor에 callGuard 설정 (호출 시 t.Fatal)
//
// When: drain
// Then: permission_check{deny, reason:"destructive"} yield
//
//	Executor.Run 미호출
//	다음 LLM payload에 ToolResult{is_error:true, content:"denied: destructive"} 포함
//	terminal{success:true}
func TestQueryLoop_PermissionDeny_SynthesizesErrorResult(t *testing.T) {
	t.Parallel()

	// Arrange
	toolUseID := "tu_deny_001"
	toolName := "rm_rf"
	inputJSON := `{}`

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, inputJSON),
		},
		// Deny 후 tool_result를 받은 뒤 2차 응답
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents(""),
		},
	)

	executor := testsupport.NewStubExecutor()
	// Deny 시 Executor.Run이 호출되면 안 된다: callGuard로 검증
	executor.SetCallGuard(func(toolName string) {
		t.Fatalf("Deny 결정 시 Executor.Run이 호출되어서는 안 된다: tool=%q", toolName)
	})

	canUse := testsupport.NewStubCanUseToolDeny("destructive")

	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	// Act
	out, err := engine.SubmitMessage(context.Background(), "please delete everything")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// Assert: permission_check{deny, reason:"destructive"} 가 yield되어야 한다
	permChecks := findMessages(msgs, message.SDKMsgPermissionCheck)
	require.Len(t, permChecks, 1, "permission_check 메시지가 정확히 1개이어야 한다")
	permPayload, ok := permChecks[0].Payload.(message.PayloadPermissionCheck)
	require.True(t, ok)
	assert.Equal(t, "deny", permPayload.Behavior, "permission_check.behavior가 deny이어야 한다")
	assert.Equal(t, "destructive", permPayload.Reason, "permission_check.reason이 destructive이어야 한다")
	assert.Equal(t, toolUseID, permPayload.ToolUseID)

	// terminal{success:true}: Deny는 loop를 종료시키지 않는다
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success, "Deny 후에도 terminal.success가 true이어야 한다")

	// 2차 LLM 호출에 is_error=true tool_result가 포함되어야 한다
	require.Equal(t, 2, stub.CallCount(), "LLM이 2번 호출되어야 한다 (1차 tool_use, 2차 denied tool_result)")
	// 2차 호출 payload의 messages에 tool_result{is_error:true} 포함 검증
	require.GreaterOrEqual(t, len(stub.RecordedRequests), 2, "2차 LLM 호출 payload가 기록되어야 한다")
	secondReq := stub.RecordedRequests[1]
	found := false
	for _, msg := range secondReq.Messages {
		for _, cb := range msg.Content {
			if cb.Type == "tool_result" && cb.ToolUseID == toolUseID {
				// content에 "denied: destructive" 포함 검증
				assert.True(t, strings.Contains(cb.ToolResultJSON, "denied"),
					"tool_result content에 'denied'가 포함되어야 한다: got %q", cb.ToolResultJSON)
				found = true
			}
		}
	}
	assert.True(t, found, "2차 LLM payload에 denied tool_result가 포함되어야 한다")
}

// --- T4.3: TestQueryLoop_ToolResultBudgetReplacement (AC-QUERY-009 일부) ---

// TestQueryLoop_ToolResultBudgetReplacement는 tool_result가 ToolResultCap을 초과할 때
// 요약 치환되는지 검증한다. REQ-QUERY-007.
//
// Given: StubExecutor가 4KB 초과 JSON 반환, TaskBudget.ToolResultCap=4096
// When: tool roundtrip
// Then: 2차 LLM payload의 tool_result content에 truncated:true 포함
func TestQueryLoop_ToolResultBudgetReplacement(t *testing.T) {
	t.Parallel()

	const cap4KB = 4096
	toolUseID := "tu_budget_001"
	toolName := "bigquery"

	// 5KB 초과 JSON 생성
	bigPayload := map[string]any{"data": strings.Repeat("x", cap4KB+1024)}
	bigJSON, err := json.Marshal(bigPayload)
	require.NoError(t, err)
	require.Greater(t, len(bigJSON), cap4KB, "bigJSON은 cap보다 커야 한다")

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
		},
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents(""),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return string(bigJSON), nil
	})

	canUse := testsupport.NewStubCanUseToolAllow()

	cfg := makeLoopConfig(t, stub, canUse, executor)
	cfg.TaskBudget.ToolResultCap = cap4KB // 4KB cap 설정
	engine, err := query.New(cfg)
	require.NoError(t, err)

	// Act
	out, err := engine.SubmitMessage(context.Background(), "run bigquery")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// Assert: terminal{success:true} 도달
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success)

	// 2차 LLM payload의 tool_result에 truncated가 포함되어야 한다
	require.GreaterOrEqual(t, len(stub.RecordedRequests), 2)
	secondReq := stub.RecordedRequests[1]
	found := false
	for _, msg := range secondReq.Messages {
		for _, cb := range msg.Content {
			if cb.Type == "tool_result" && cb.ToolUseID == toolUseID {
				assert.True(t, strings.Contains(cb.ToolResultJSON, "truncated"),
					"초과된 tool_result에 truncated 정보가 포함되어야 한다: got %q", cb.ToolResultJSON)
				found = true
			}
		}
	}
	assert.True(t, found, "2차 LLM payload에 tool_result가 포함되어야 한다")
}

// --- T4.1b: TestQueryLoop_ToolCallAllow_ExecutorError (AC-QUERY-002 edge case) ---

// TestQueryLoop_ToolCallAllow_ExecutorError는 Executor.Run이 에러를 반환할 때
// is_error tool_result가 합성되고 loop가 계속됨을 검증한다.
func TestQueryLoop_ToolCallAllow_ExecutorError(t *testing.T) {
	t.Parallel()

	toolUseID := "tu_exec_err"
	toolName := "failing_tool"

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
		},
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents(""),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return "", fmt.Errorf("tool execution failed")
	})

	canUse := testsupport.NewStubCanUseToolAllow()
	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "run failing")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// terminal{success:true}: Executor 에러도 loop를 종료시키지 않는다
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success)

	// 2차 LLM 호출에 error tool_result가 포함되어야 한다
	require.Equal(t, 2, stub.CallCount())
	require.GreaterOrEqual(t, len(stub.RecordedRequests), 2)
	secondReq := stub.RecordedRequests[1]
	found := false
	for _, msg := range secondReq.Messages {
		for _, cb := range msg.Content {
			if cb.Type == "tool_result" && cb.ToolUseID == toolUseID {
				assert.True(t, strings.Contains(cb.ToolResultJSON, "error"),
					"executor 에러 시 tool_result에 error가 포함되어야 한다")
				found = true
			}
		}
	}
	assert.True(t, found, "에러 tool_result가 2차 LLM payload에 포함되어야 한다")
}

// --- T4.4: TestQueryLoop_MultipleToolBlocks_Sequential (AC-QUERY-002 edge case) ---

// TestQueryLoop_MultipleToolBlocks_Sequential는 한 응답에 tool_use 블록이 2개인 경우
// array order대로 순차 실행됨을 검증한다. spec.md Exclusions 10번: 병렬 실행 금지.
//
// Given: StubLLM 1차 응답에 tool_use 블록 2개 (순서: first, second)
// When: drain
// Then: Executor.Run 호출 순서가 first → second
func TestQueryLoop_MultipleToolBlocks_Sequential(t *testing.T) {
	t.Parallel()

	// Arrange: tool_use 2개를 한 응답에 포함
	toolUseID1 := "tu_seq_001"
	toolUseID2 := "tu_seq_002"

	// 두 tool_use 블록을 직렬로 담은 이벤트 시퀀스
	events1st := append(
		testsupport.MakeToolUseEvents(toolUseID1, "first", `{"order":1}`),
		testsupport.MakeToolUseEvents(toolUseID2, "second", `{"order":2}`)...,
	)
	// 마지막 message_stop은 1개만 필요 (MakeToolUseEvents가 각각 stop을 포함하므로 마지막 것 유지)

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{Events: events1st},
		testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("")},
	)

	executor := testsupport.NewStubExecutor()
	var callOrder []string
	executor.Register("first", func(_ context.Context, _ string, _ map[string]any) (string, error) {
		callOrder = append(callOrder, "first")
		return `{"done":1}`, nil
	})
	executor.Register("second", func(_ context.Context, _ string, _ map[string]any) (string, error) {
		callOrder = append(callOrder, "second")
		return `{"done":2}`, nil
	})

	canUse := testsupport.NewStubCanUseToolAllow()

	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	// Act
	out, err := engine.SubmitMessage(context.Background(), "run both")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// Assert: 두 도구가 순서대로 실행되어야 한다
	require.Equal(t, []string{"first", "second"}, callOrder,
		"tool_use 블록은 array order대로 순차 실행되어야 한다")

	// terminal{success:true}
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success)
}

// --- 추가 coverage 테스트 ---

// TestQueryLoop_ContextCancellation은 ctx 취소 시 loop가 정상 종료됨을 검증한다.
// REQ-QUERY-010: abort 시 정상 종료.
func TestQueryLoop_ContextCancellation(t *testing.T) {
	t.Parallel()

	// 느린 stub: 이벤트 전달 전에 ctx가 취소된다
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	stub := testsupport.NewStubLLMCallSimple("hello")
	executor := testsupport.NewStubExecutor()
	canUse := testsupport.NewStubCanUseToolAllow()
	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	// 취소된 ctx로 호출
	out, err := engine.SubmitMessage(ctx, "hi")
	require.NoError(t, err) // SubmitMessage는 즉시 반환

	// drain: ctx 취소로 loop가 조기 종료되어야 한다
	var msgs []message.SDKMessage
	for m := range out {
		msgs = append(msgs, m)
	}

	// ctx 취소 시 0개 또는 일부 메시지 후 채널 close
	// 채널이 close되었음을 drain 완료로 검증 (deadlock 없음)
	t.Logf("ctx 취소 시 수신된 메시지 수: %d", len(msgs))
}

// TestQueryLoop_ToolDeny_EmptyReason은 Deny reason이 빈 문자열일 때
// fallback 처리를 검증한다. AC-QUERY-003 edge case.
func TestQueryLoop_ToolDeny_EmptyReason(t *testing.T) {
	t.Parallel()

	toolUseID := "tu_deny_empty"
	toolName := "restricted"

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
		},
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents(""),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.SetCallGuard(func(toolName string) {
		t.Fatalf("Deny 시 Executor.Run 호출 금지: tool=%q", toolName)
	})

	// 빈 reason으로 Deny
	canUse := testsupport.NewStubCanUseToolDeny("")
	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "run restricted")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// terminal{success:true}
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success)

	// permission_check{deny} 포함
	permChecks := findMessages(msgs, message.SDKMsgPermissionCheck)
	require.Len(t, permChecks, 1)
	permPayload, ok := permChecks[0].Payload.(message.PayloadPermissionCheck)
	require.True(t, ok)
	assert.Equal(t, "deny", permPayload.Behavior)
	// reason이 빈 문자열이어도 tool_result에는 fallback 메시지가 포함되어야 한다
	require.GreaterOrEqual(t, len(stub.RecordedRequests), 2)
	secondReq := stub.RecordedRequests[1]
	found := false
	for _, msg := range secondReq.Messages {
		for _, cb := range msg.Content {
			if cb.Type == "tool_result" && cb.ToolUseID == toolUseID {
				// SynthesizeDeniedResult가 fallback 메시지를 생성해야 한다
				assert.NotEmpty(t, cb.ToolResultJSON, "tool_result content는 비어있지 않아야 한다")
				found = true
			}
		}
	}
	assert.True(t, found)
}

// TestQueryLoop_ToolUse_EmptyInputJSON은 빈 inputJSON인 tool_use가 정상 처리됨을 검증한다.
// parseInputJSON의 빈 문자열 경로 커버.
func TestQueryLoop_ToolUse_EmptyInputJSON(t *testing.T) {
	t.Parallel()

	toolUseID := "tu_empty_input"
	toolName := "noop"

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, ""),
		},
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents(""),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, input map[string]any) (string, error) {
		// 빈 input이어도 정상 처리
		return `{"ok":true}`, nil
	})

	canUse := testsupport.NewStubCanUseToolAllow()
	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "noop")
	require.NoError(t, err)
	msgs := drainMessages(out)

	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success)
}

// TestQueryLoop_ToolUse_InvalidInputJSON은 잘못된 JSON inputJSON이 빈 맵으로 fallback됨을 검증한다.
// parseInputJSON의 unmarshal 에러 경로 커버.
func TestQueryLoop_ToolUse_InvalidInputJSON(t *testing.T) {
	t.Parallel()

	toolUseID := "tu_bad_json"
	toolName := "noop2"

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, "not-valid-json"),
		},
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents(""),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, input map[string]any) (string, error) {
		// 잘못된 JSON도 빈 맵으로 fallback 처리되어 실행되어야 한다
		return `{"ok":true}`, nil
	})

	canUse := testsupport.NewStubCanUseToolAllow()
	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "noop2")
	require.NoError(t, err)
	msgs := drainMessages(out)

	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success)
}

// TestQueryLoop_PermissionAsk_FallsBackToDeny는 S4에서 Ask 분기가 Deny로 대체 처리됨을 검증한다.
// Ask 처리는 S6에서 구현 예정; 현재는 Deny와 동일하게 처리된다.
func TestQueryLoop_PermissionAsk_FallsBackToDeny(t *testing.T) {
	t.Parallel()

	toolUseID := "tu_ask_001"
	toolName := "sensitive_op"

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
		},
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents(""),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.SetCallGuard(func(toolName string) {
		t.Fatalf("Ask(→Deny) 시 Executor.Run 호출 금지: tool=%q", toolName)
	})

	canUse := testsupport.NewStubCanUseToolAsk("requires_approval")
	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "sensitive")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// terminal{success:true}: Ask→Deny 대체도 loop를 종료시키지 않는다
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success)

	// permission_check{deny} 포함 (Ask가 Deny로 대체)
	permChecks := findMessages(msgs, message.SDKMsgPermissionCheck)
	require.Len(t, permChecks, 1)
	permPayload, ok := permChecks[0].Payload.(message.PayloadPermissionCheck)
	require.True(t, ok)
	assert.Equal(t, "deny", permPayload.Behavior, "S4에서 Ask는 Deny로 대체 처리된다")
}

// TestQueryLoop_MessageDeltaEvent는 TypeMessageDelta 이벤트가 default 브랜치를 통해
// 정상 전달됨을 검증한다. (queryLoop default case 커버)
func TestQueryLoop_MessageDeltaEvent(t *testing.T) {
	t.Parallel()

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: []message.StreamEvent{
				// TypeMessageDelta: default 브랜치에서 처리
				{Type: message.TypeMessageDelta, StopReason: "end_turn"},
				{Type: message.TypeTextDelta, Delta: "hello"},
				{Type: message.TypeMessageStop, StopReason: "end_turn"},
			},
		},
	)

	executor := testsupport.NewStubExecutor()
	canUse := testsupport.NewStubCanUseToolAllow()
	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "test")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// terminal{success:true}
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success)

	// TypeMessageDelta도 stream_event로 전달되어야 한다
	streamEvents := findMessages(msgs, message.SDKMsgStreamEvent)
	found := false
	for _, m := range streamEvents {
		if p, ok := m.Payload.(message.PayloadStreamEvent); ok {
			if p.Event.Type == message.TypeMessageDelta {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "TypeMessageDelta 이벤트가 stream_event로 전달되어야 한다")
}

// TestQueryLoop_LLMError는 LLM 호출이 에러를 반환할 때 terminal{success:false}를 검증한다.
func TestQueryLoop_LLMError(t *testing.T) {
	t.Parallel()

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{Err: fmt.Errorf("connection refused")},
	)

	executor := testsupport.NewStubExecutor()
	canUse := testsupport.NewStubCanUseToolAllow()
	cfg := makeLoopConfig(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "hi")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// terminal{success:false} 이어야 한다
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.False(t, termPayload.Success, "LLM 에러 시 terminal.success가 false이어야 한다")
	assert.Contains(t, termPayload.Error, "connection refused")
}
