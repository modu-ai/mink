//go:build integration

// Package loop_test — SPEC-GOOSE-QUERY-001 S6 통합 테스트.
// T6.1: Ask 분기 → permission_request 송출 + loop suspend (AC-QUERY-013)
// T6.2: ResolvePermission(Allow) → loop 재개 + 정상 terminal
// T6.3: Edge cases — resolve_deny, cancel_while_pending, multiple_asks_fifo
//
// 빌드 태그: integration (go test -tags=integration)
package loop_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/permissions"
	"github.com/modu-ai/mink/internal/query"
	"github.com/modu-ai/mink/internal/query/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// makeS6Config는 S6 테스트용 기본 QueryEngineConfig를 생성한다.
func makeS6Config(t *testing.T, stub *testsupport.StubLLMCall, canUse permissions.CanUseTool, executor query.Executor) query.QueryEngineConfig {
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

// --- T6.1 + T6.2: TestQueryLoop_AskPermission_SuspendResume (AC-QUERY-013) ---

// TestQueryLoop_AskPermission_SuspendResume는 Ask permission 분기에서
// permission_request yield → loop suspend → ResolvePermission(Allow) → loop 재개 → terminal{success:true}
// 전체 흐름을 검증한다.
//
// Given: stub LLM 1차 = tool_use{deleteFile}, StubCanUseTool = Ask("destructive op")
//
//	payload recorder 활성, StubExecutor resolve 전 호출 시 t.Fatal
//
// When: SubmitMessage drain 중 permission_request 수신 후 50ms 뒤 ResolvePermission(Allow)
// Then: (a) resolve 전 LLM call 건수 == 1 (suspend 확증)
//
//	(b) resolve 후 tool 실행 → tool_result → 2nd LLM call
//	(c) 2nd LLM call messages[]에 tool_result 포함
//	(d) terminal{success:true}
func TestQueryLoop_AskPermission_SuspendResume(t *testing.T) {
	t.Parallel()

	const toolUseID = "tu_ask_001"
	const toolName = "deleteFile"
	inputJSON := `{"path":"/tmp/x"}`

	// Arrange
	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, inputJSON),
		},
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("done"),
		},
	)

	canUse := testsupport.NewStubCanUseToolAsk("destructive op")

	executor := testsupport.NewStubExecutor()
	var executorCalledMu sync.Mutex
	executorCalledBeforeResolve := false
	resolveHappened := false

	executor.Register(toolName, func(ctx context.Context, tuID string, input map[string]any) (string, error) {
		executorCalledMu.Lock()
		if !resolveHappened {
			executorCalledBeforeResolve = true
		}
		executorCalledMu.Unlock()
		return `{"deleted":true}`, nil
	})

	cfg := makeS6Config(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	out, err := engine.SubmitMessage(ctx, "del file")
	require.NoError(t, err)

	// drain까지의 메시지를 수집한다.
	// permission_request 수신 → 50ms 후 ResolvePermission(Allow)
	var msgs []message.SDKMessage
	var permReqMsg message.PayloadPermissionRequest

	for msg := range out {
		msgs = append(msgs, msg)

		if msg.Type == message.SDKMsgPermissionRequest {
			payload, ok := msg.Payload.(message.PayloadPermissionRequest)
			require.True(t, ok, "permission_request payload 타입 불일치")
			permReqMsg = payload

			// (a) resolve 전 LLM call 건수 == 1 검증
			assert.Equal(t, 1, stub.CallCount(), "suspend 중 추가 LLM 호출 없어야 함")

			// 50ms 후 별도 goroutine에서 ResolvePermission(Allow)
			go func() {
				time.Sleep(50 * time.Millisecond)
				executorCalledMu.Lock()
				resolveHappened = true
				executorCalledMu.Unlock()
				err := engine.ResolvePermission(permReqMsg.ToolUseID, int(permissions.Allow), "")
				assert.NoError(t, err)
			}()
		}
	}

	// (a) resolve 전 executor 호출 없었음을 확증
	assert.False(t, executorCalledBeforeResolve, "resolve 전에 executor가 호출되면 안 됨")

	// (b) permission_request 메시지가 수신되었음을 확인
	permReqMsgs := findMessages(msgs, message.SDKMsgPermissionRequest)
	require.Len(t, permReqMsgs, 1, "permission_request 메시지 1건 필요")
	pReq := permReqMsgs[0].Payload.(message.PayloadPermissionRequest)
	assert.Equal(t, toolUseID, pReq.ToolUseID)
	assert.Equal(t, toolName, pReq.ToolName)

	// (c) 2nd LLM call이 수행되었음을 확인 (총 2회)
	assert.Equal(t, 2, stub.CallCount(), "resolve 후 2nd LLM call 필요")

	// (d) 2nd LLM call messages[]에 tool_result 포함 확인
	stub.RecordMu().Lock()
	reqs := stub.RecordedRequests
	stub.RecordMu().Unlock()
	require.Len(t, reqs, 2, "LLM call 2회 필요")
	secondCallMsgs := reqs[1].Messages
	require.NotEmpty(t, secondCallMsgs)
	lastMsg := secondCallMsgs[len(secondCallMsgs)-1]
	assert.Equal(t, "user", lastMsg.Role)
	hasToolResult := false
	for _, blk := range lastMsg.Content {
		if blk.Type == "tool_result" && blk.ToolUseID == toolUseID {
			hasToolResult = true
			break
		}
	}
	assert.True(t, hasToolResult, "2nd LLM call messages에 tool_result 포함 필요")

	// (e) terminal{success:true}
	termMsgs := findMessages(msgs, message.SDKMsgTerminal)
	require.Len(t, termMsgs, 1)
	term := termMsgs[0].Payload.(message.PayloadTerminal)
	assert.True(t, term.Success)
}

// --- T6.3 edge cases ---

// TestQueryLoop_AskPermission_ResolveDeny는 Deny 결정 시
// ToolResult{is_error:true, content:"denied: destructive op"} 합성 후 대화 계속 진행을 검증한다.
func TestQueryLoop_AskPermission_ResolveDeny(t *testing.T) {
	t.Parallel()

	const toolUseID = "tu_ask_deny"
	const toolName = "deleteFile"

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, `{"path":"/tmp/y"}`),
		},
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("ok after deny"),
		},
	)

	canUse := testsupport.NewStubCanUseToolAsk("destructive op")

	// Deny 시 executor는 절대 호출되면 안 됨
	executor := testsupport.NewStubExecutor()
	executor.SetCallGuard(func(toolName string) {
		t.Fatalf("resolve_deny 시나리오: executor.Run(%q) 호출 금지", toolName)
	})

	cfg := makeS6Config(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	out, err := engine.SubmitMessage(ctx, "please delete")
	require.NoError(t, err)

	var msgs []message.SDKMessage
	for msg := range out {
		msgs = append(msgs, msg)
		if msg.Type == message.SDKMsgPermissionRequest {
			go func() {
				time.Sleep(20 * time.Millisecond)
				err := engine.ResolvePermission(toolUseID, int(permissions.Deny), "destructive op")
				assert.NoError(t, err)
			}()
		}
	}

	// terminal{success:true} (대화 계속 진행)
	termMsgs := findMessages(msgs, message.SDKMsgTerminal)
	require.Len(t, termMsgs, 1)
	term := termMsgs[0].Payload.(message.PayloadTerminal)
	assert.True(t, term.Success)

	// 2nd LLM call messages[]에 is_error tool_result 포함 확인
	stub.RecordMu().Lock()
	reqs := stub.RecordedRequests
	stub.RecordMu().Unlock()
	require.Len(t, reqs, 2)
	secondCallMsgs := reqs[1].Messages
	lastMsg := secondCallMsgs[len(secondCallMsgs)-1]
	hasDeniedResult := false
	for _, blk := range lastMsg.Content {
		if blk.Type == "tool_result" && blk.ToolUseID == toolUseID {
			hasDeniedResult = true
			break
		}
	}
	assert.True(t, hasDeniedResult, "deny 결과 tool_result가 2nd call messages에 포함되어야 함")
}

// TestQueryLoop_AskPermission_CancelWhilePending는 ResolvePermission 전 ctx.Cancel 시
// abort 우선으로 loop가 종료됨을 검증한다. (REQ-010)
func TestQueryLoop_AskPermission_CancelWhilePending(t *testing.T) {
	t.Parallel()

	const toolUseID = "tu_ask_cancel"
	const toolName = "deleteFile"

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
		},
	)

	canUse := testsupport.NewStubCanUseToolAsk("destructive op")
	executor := testsupport.NewStubExecutor()
	executor.SetCallGuard(func(toolName string) {
		t.Fatalf("cancel_while_pending: executor.Run(%q) 호출 금지", toolName)
	})

	cfg := makeS6Config(t, stub, canUse, executor)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	out, err := engine.SubmitMessage(ctx, "del")
	require.NoError(t, err)

	var msgs []message.SDKMessage
	for msg := range out {
		msgs = append(msgs, msg)
		if msg.Type == message.SDKMsgPermissionRequest {
			// ResolvePermission 호출 없이 ctx cancel
			go func() {
				time.Sleep(20 * time.Millisecond)
				cancel()
			}()
		}
	}

	// loop가 abort로 종료되어야 함 (terminal 없이 채널 close 또는 terminal{success:false})
	// ctx 취소 시 loop는 out 채널을 close하므로 range가 종료됨
	// executor는 호출되지 않아야 함 (SetCallGuard가 보장)
	termMsgs := findMessages(msgs, message.SDKMsgTerminal)
	if len(termMsgs) > 0 {
		term := termMsgs[0].Payload.(message.PayloadTerminal)
		assert.False(t, term.Success, "abort 시 success=false 또는 terminal 없이 종료")
	}
	// ctx cancel 후 executor 호출 0건 (callGuard가 t.Fatal)
}

// TestQueryLoop_AskPermission_MultipleAsksFIFO는 동일 turn 내 Ask 2건이 있을 때
// FIFO 순서로 inbox에 deliver되어 두 tool이 모두 올바르게 처리됨을 검증한다.
func TestQueryLoop_AskPermission_MultipleAsksFIFO(t *testing.T) {
	t.Parallel()

	const toolUseID1 = "tu_ask_fifo_1"
	const toolUseID2 = "tu_ask_fifo_2"
	const toolName = "deleteFile"

	// 동일 응답에 tool_use 2개
	twoToolEvents := append(
		testsupport.MakeToolUseEvents(toolUseID1, toolName, `{"n":1}`),
		testsupport.MakeToolUseEvents(toolUseID2, toolName, `{"n":2}`)...,
	)
	// 마지막 message_stop은 1개만 필요 — MakeToolUseEvents가 각각 포함하므로 하나 제거
	// 실제로는 stubs가 이벤트를 순서대로 전달하므로 중복 message_stop은 별도로 필터 없이 괜찮음.
	// StubLLMCall은 Events를 순서대로 전달한다.

	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{Events: twoToolEvents},
		testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("all done")},
	)

	canUse := testsupport.NewStubCanUseToolAsk("destructive op")
	executor := testsupport.NewStubExecutor()

	var executionOrder []string
	var orderMu sync.Mutex
	executor.Register(toolName, func(ctx context.Context, tuID string, input map[string]any) (string, error) {
		orderMu.Lock()
		executionOrder = append(executionOrder, tuID)
		orderMu.Unlock()
		return `{"ok":true}`, nil
	})

	cfg := makeS6Config(t, stub, canUse, executor)
	// permInbox는 buffered(cap 4) — 2개 Ask가 동시에 대기 가능해야 함
	engine, err := query.New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	out, err := engine.SubmitMessage(ctx, "del all")
	require.NoError(t, err)

	var msgs []message.SDKMessage

	// loop는 Ask tool을 순차적으로 처리한다:
	// 1번째 permission_request 수신 → loop suspend → resolve 1 → loop 재개
	// → 2번째 permission_request 수신 → loop suspend → resolve 2 → loop 재개 → terminal
	// 따라서 permission_request를 받는 즉시 resolve해야 다음 permission_request가 온다.
	for msg := range out {
		msgs = append(msgs, msg)
		if msg.Type == message.SDKMsgPermissionRequest {
			payload := msg.Payload.(message.PayloadPermissionRequest)
			// 받는 즉시 goroutine에서 resolve — FIFO 순서는 수신 순서와 동일
			tuID := payload.ToolUseID
			go func() {
				time.Sleep(10 * time.Millisecond)
				err := engine.ResolvePermission(tuID, int(permissions.Allow), "")
				assert.NoError(t, err)
			}()
		}
	}

	// terminal{success:true}
	termMsgs := findMessages(msgs, message.SDKMsgTerminal)
	require.Len(t, termMsgs, 1)
	term := termMsgs[0].Payload.(message.PayloadTerminal)
	assert.True(t, term.Success)

	// FIFO 순서 검증: tu_ask_fifo_1이 먼저 실행되어야 함
	orderMu.Lock()
	defer orderMu.Unlock()
	require.Len(t, executionOrder, 2, "두 tool이 모두 실행되어야 함")
	assert.Equal(t, toolUseID1, executionOrder[0], "FIFO: tu_ask_fifo_1이 먼저 실행")
	assert.Equal(t, toolUseID2, executionOrder[1], "FIFO: tu_ask_fifo_2가 나중 실행")
}

// TestQueryLoop_ResolvePermission_UnknownID는 알 수 없는 toolUseID로
// ResolvePermission 호출 시 ErrUnknownPermissionRequest 반환을 검증한다.
func TestQueryLoop_ResolvePermission_UnknownID(t *testing.T) {
	t.Parallel()

	stub := testsupport.NewStubLLMCallSimple("ok")
	cfg := makeS6Config(t, stub, testsupport.NewStubCanUseToolAllow(), testsupport.NewStubExecutor())
	engine, err := query.New(cfg)
	require.NoError(t, err)

	// loop가 실행 중이지 않아도 unknown ID는 에러를 반환해야 한다.
	err = engine.ResolvePermission("nonexistent_id", int(permissions.Allow), "")
	assert.ErrorIs(t, err, query.ErrUnknownPermissionRequest, "unknown toolUseID는 ErrUnknownPermissionRequest 반환")
}
