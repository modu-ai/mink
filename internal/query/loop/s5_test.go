//go:build integration

// Package loop_test — SPEC-GOOSE-QUERY-001 S5 통합 테스트.
// T5.1: TaskBudget 소진 → budget_exceeded (AC-QUERY-006)
// T5.2: MaxTurns 도달 → max_turns terminal (AC-QUERY-007)
// T5.3: 2턴 연속 SubmitMessage messages 누적 (AC-QUERY-004)
// T5.4: max_output_tokens 재시도 ≤ 3회 (AC-QUERY-005)
// T5.5: budget gate vs max_turns 교차 검증
//
// 빌드 태그: integration (go test -tags=integration)
package loop_test

import (
	"context"
	"testing"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"github.com/modu-ai/goose/internal/query/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// makeS5Config는 S5 테스트용 기본 QueryEngineConfig를 생성한다.
func makeS5Config(t *testing.T, stub *testsupport.StubLLMCall) query.QueryEngineConfig {
	t.Helper()
	return query.QueryEngineConfig{
		LLMCall:    stub.AsFunc(),
		Tools:      []query.ToolDefinition{},
		CanUseTool: testsupport.NewStubCanUseToolAllow(),
		Executor:   testsupport.NewStubExecutor(),
		Logger:     zaptest.NewLogger(t),
		MaxTurns:   10,
		TaskBudget: query.TaskBudget{Total: 10000, Remaining: 10000, ToolResultCap: 0},
	}
}

// --- T5.1: TestQueryLoop_BudgetExhausted (AC-QUERY-006) ---

// TestQueryLoop_BudgetExhausted는 TaskBudget.Remaining이 소진될 때
// budget_exceeded terminal을 검증한다.
//
// Given: TaskBudget{Total:50, Remaining:50}, stub LLM 1턴차에 60 units 소비
//
//	1턴차 응답은 tool_use를 포함하여 2턴차를 유발
//
// When: SubmitMessage drain
// Then: 1턴 완료 후 Remaining=-10, 2턴차 iteration 시작 시 budget_exceeded terminal{success:false}
func TestQueryLoop_BudgetExhausted(t *testing.T) {
	t.Parallel()

	// Arrange: 1턴차에서 60 units 소비 (50 budget 초과)
	// tool_use를 포함하여 2턴차 시작을 유발한다.
	// 2턴차 iteration 시작 시 remaining=-10 <= 0 gate가 발동해야 한다.
	toolUseID := "tu_budget_burn"
	toolName := "burn"
	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events:            testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
			UsageInputTokens:  30,
			UsageOutputTokens: 30, // 합계 60, budget 50 초과
		},
		// 2번째 응답은 도달해서는 안 된다 (budget_exceeded로 종료)
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("should not reach"),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true}`, nil
	})
	canUse := testsupport.NewStubCanUseToolAllow()

	cfg := makeS5Config(t, stub)
	cfg.TaskBudget = query.TaskBudget{Total: 50, Remaining: 50}
	cfg.MaxTurns = 10
	cfg.CanUseTool = canUse
	cfg.Executor = executor

	engine, err := query.New(cfg)
	require.NoError(t, err)

	// Act: 1턴 완료 후 budget 소진 → 2턴차 시작 시 budget_exceeded
	out, err := engine.SubmitMessage(context.Background(), "burn budget")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// Assert: terminal{success:false, error:"budget_exceeded"}
	require.NotEmpty(t, msgs)
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type, "마지막은 terminal이어야 한다")
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.False(t, termPayload.Success, "budget 소진 시 success는 false이어야 한다")
	assert.Equal(t, "budget_exceeded", termPayload.Error, "error는 budget_exceeded이어야 한다")

	// LLM이 정확히 1번만 호출되어야 한다 (2번째 응답 미도달)
	assert.Equal(t, 1, stub.CallCount(), "budget 소진 시 LLM은 1번만 호출되어야 한다")

	t.Run("total_zero", func(t *testing.T) {
		// Total=0이면 첫 turn 즉시 budget_exceeded (LLM 호출 없음)
		stubZero := testsupport.NewStubLLMCall(
			testsupport.StubLLMResponse{
				Events:            testsupport.MakeStopEvents("won't matter"),
				UsageInputTokens:  1,
				UsageOutputTokens: 1,
			},
		)
		cfgZero := makeS5Config(t, stubZero)
		cfgZero.TaskBudget = query.TaskBudget{Total: 0, Remaining: 0}
		cfgZero.MaxTurns = 10

		engZero, err := query.New(cfgZero)
		require.NoError(t, err)

		outZero, err := engZero.SubmitMessage(context.Background(), "zero budget")
		require.NoError(t, err)
		msgsZero := drainMessages(outZero)

		lastZero := msgsZero[len(msgsZero)-1]
		require.Equal(t, message.SDKMsgTerminal, lastZero.Type)
		termZero, ok := lastZero.Payload.(message.PayloadTerminal)
		require.True(t, ok)
		assert.False(t, termZero.Success)
		assert.Equal(t, "budget_exceeded", termZero.Error)
		// Total=0이면 LLM 호출 없이 즉시 종료
		assert.Equal(t, 0, stubZero.CallCount(), "Total=0이면 LLM 호출 없이 종료되어야 한다")
	})
}

// --- T5.2: TestQueryLoop_MaxTurnsReached (AC-QUERY-007) ---

// TestQueryLoop_MaxTurnsReached는 MaxTurns 도달 시
// max_turns terminal{success:true}을 검증한다.
//
// Given: MaxTurns=2, stub LLM 매 응답 tool_use{loop} → 무한 loop 가능성
// When: drain
// Then: 2턴 후 terminal{success:true, error:"max_turns"}
func TestQueryLoop_MaxTurnsReached(t *testing.T) {
	t.Parallel()

	// Arrange: 매 응답 tool_use → 무한 loop 가능. MaxTurns=2로 제한.
	toolUseID := "tu_loop"
	toolName := "loop_tool"

	stub := testsupport.NewStubLLMCall(
		// 1턴차: tool_use → 2턴차 유발
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID+"_1", toolName, `{"i":1}`),
		},
		// 2턴차: tool_use → 3턴차 시도하지만 MaxTurns=2 gate 발동
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID+"_2", toolName, `{"i":2}`),
		},
		// 3번째 응답은 도달해서는 안 된다
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("should not reach"),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true}`, nil
	})
	canUse := testsupport.NewStubCanUseToolAllow()

	cfg := makeS5Config(t, stub)
	cfg.MaxTurns = 2
	cfg.CanUseTool = canUse
	cfg.Executor = executor
	cfg.TaskBudget = query.TaskBudget{Total: 100000, Remaining: 100000}

	engine, err := query.New(cfg)
	require.NoError(t, err)

	// Act
	out, err := engine.SubmitMessage(context.Background(), "loop forever")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// Assert: terminal{success:true, error:"max_turns"}
	require.NotEmpty(t, msgs)
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success, "max_turns 도달은 success=true이어야 한다 (AC-QUERY-007)")
	assert.Equal(t, "max_turns", termPayload.Error, "error는 max_turns이어야 한다")

	// LLM이 정확히 2번 호출되어야 한다
	assert.Equal(t, 2, stub.CallCount(), "MaxTurns=2이면 LLM이 2번 호출되어야 한다")
}

// --- T5.3: TestQueryEngine_TwoSubmitMessages_ShareMessages (AC-QUERY-004) ---

// TestQueryEngine_TwoSubmitMessages_ShareMessages는 동일 engine 인스턴스에서
// 2번 연속 SubmitMessage 시 messages가 누적됨을 검증한다.
//
// Given: 동일 engine, stub LLM 각 호출 stop
// When: SubmitMessage("first") drain → SubmitMessage("second") drain
// Then: 2번째 호출의 첫 API call payload의 messages[]에 1턴차 user+assistant 포함
func TestQueryEngine_TwoSubmitMessages_ShareMessages(t *testing.T) {
	t.Parallel()

	// Arrange: 각 SubmitMessage 호출에 대해 단순 stop 응답
	stub := testsupport.NewStubLLMCall(
		// 1번째 SubmitMessage 응답
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("first response"),
		},
		// 2번째 SubmitMessage 응답
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("second response"),
		},
	)

	cfg := makeS5Config(t, stub)
	engine, err := query.New(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// Act: 첫 번째 SubmitMessage
	out1, err := engine.SubmitMessage(ctx, "first")
	require.NoError(t, err)
	_ = drainMessages(out1) // drain 완료 대기

	// Act: 두 번째 SubmitMessage
	out2, err := engine.SubmitMessage(ctx, "second")
	require.NoError(t, err)
	_ = drainMessages(out2) // drain 완료 대기

	// Assert: 두 번째 LLM 호출의 messages에 1턴차 user+assistant 포함
	require.GreaterOrEqual(t, len(stub.RecordedRequests), 2,
		"LLM이 2번 이상 호출되어야 한다")

	secondReq := stub.RecordedRequests[1]
	// 2번째 호출의 messages에는 최소:
	// [0] 1턴차 user("first"), [1] 1턴차 assistant, [2] 2턴차 user("second")
	require.GreaterOrEqual(t, len(secondReq.Messages), 3,
		"2번째 호출 messages에는 이전 turn의 user+assistant + 새 user가 포함되어야 한다")

	// messages[0]은 1턴차 user("first")
	assert.Equal(t, "user", secondReq.Messages[0].Role,
		"messages[0]은 1턴차 user이어야 한다")
	require.NotEmpty(t, secondReq.Messages[0].Content)
	assert.Equal(t, "first", secondReq.Messages[0].Content[0].Text,
		"messages[0].content[0].text는 'first'이어야 한다")

	// messages[1]은 1턴차 assistant
	assert.Equal(t, "assistant", secondReq.Messages[1].Role,
		"messages[1]은 1턴차 assistant이어야 한다")

	// messages[2]은 2턴차 user("second")
	assert.Equal(t, "user", secondReq.Messages[2].Role,
		"messages[2]은 2턴차 user이어야 한다")
	require.NotEmpty(t, secondReq.Messages[2].Content)
	assert.Equal(t, "second", secondReq.Messages[2].Content[0].Text,
		"messages[2].content[0].text는 'second'이어야 한다")

	// 최소: 2번째 LLM 호출에 3개 이상 messages (user1 + assistant1 + user2)
	assert.GreaterOrEqual(t, len(secondReq.Messages), 3)
}

// --- T5.4: TestQueryLoop_MaxOutputTokens_Retries (AC-QUERY-005) ---

// TestQueryLoop_MaxOutputTokens_Retries는 max_output_tokens 재시도 로직을 검증한다.
//
// SubTest 1: 3회 재시도 후 4번째 정상 응답 → terminal{success:true}
// SubTest 2: 4회 모두 max_output_tokens → terminal{success:false, error:"max_output_tokens_exhausted"}
func TestQueryLoop_MaxOutputTokens_Retries(t *testing.T) {
	t.Parallel()

	t.Run("recover_on_4th", func(t *testing.T) {
		t.Parallel()
		// Arrange: 첫 3회 max_output_tokens, 4번째 정상
		stub := testsupport.NewStubLLMCall(
			testsupport.StubLLMResponse{
				Events: testsupport.MakeMaxOutputTokensEvents("partial 1"),
			},
			testsupport.StubLLMResponse{
				Events: testsupport.MakeMaxOutputTokensEvents("partial 2"),
			},
			testsupport.StubLLMResponse{
				Events: testsupport.MakeMaxOutputTokensEvents("partial 3"),
			},
			// 4번째: 정상 stop
			testsupport.StubLLMResponse{
				Events: testsupport.MakeStopEvents("complete"),
			},
		)

		cfg := makeS5Config(t, stub)
		cfg.MaxTurns = 10
		cfg.TaskBudget = query.TaskBudget{Total: 100000, Remaining: 100000}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out, err := engine.SubmitMessage(context.Background(), "generate long")
		require.NoError(t, err)
		msgs := drainMessages(out)

		last := msgs[len(msgs)-1]
		require.Equal(t, message.SDKMsgTerminal, last.Type)
		termPayload, ok := last.Payload.(message.PayloadTerminal)
		require.True(t, ok)
		assert.True(t, termPayload.Success, "3회 재시도 후 성공 시 success=true이어야 한다")
		assert.Equal(t, "", termPayload.Error, "성공 시 error는 빈 문자열이어야 한다")

		// LLM 호출 4번 (3회 retry + 1회 성공)
		assert.Equal(t, 4, stub.CallCount(), "3회 retry + 1회 성공 = 4번 LLM 호출이어야 한다")
	})

	t.Run("exhaust_after_3", func(t *testing.T) {
		t.Parallel()
		// Arrange: 4회 모두 max_output_tokens
		stub := testsupport.NewStubLLMCall(
			testsupport.StubLLMResponse{
				Events: testsupport.MakeMaxOutputTokensEvents("partial 1"),
			},
			testsupport.StubLLMResponse{
				Events: testsupport.MakeMaxOutputTokensEvents("partial 2"),
			},
			testsupport.StubLLMResponse{
				Events: testsupport.MakeMaxOutputTokensEvents("partial 3"),
			},
			testsupport.StubLLMResponse{
				Events: testsupport.MakeMaxOutputTokensEvents("partial 4"),
			},
		)

		cfg := makeS5Config(t, stub)
		cfg.MaxTurns = 10
		cfg.TaskBudget = query.TaskBudget{Total: 100000, Remaining: 100000}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out, err := engine.SubmitMessage(context.Background(), "generate long")
		require.NoError(t, err)
		msgs := drainMessages(out)

		last := msgs[len(msgs)-1]
		require.Equal(t, message.SDKMsgTerminal, last.Type)
		termPayload, ok := last.Payload.(message.PayloadTerminal)
		require.True(t, ok)
		assert.False(t, termPayload.Success, "4회 모두 max_output_tokens 시 success=false이어야 한다")
		assert.Equal(t, "max_output_tokens_exhausted", termPayload.Error,
			"error는 max_output_tokens_exhausted이어야 한다")

		// LLM 호출 4번 (초기 1회 + 재시도 3회)
		assert.Equal(t, 4, stub.CallCount(), "4번 LLM 호출 (초기+재시도3) 이어야 한다")
	})
}

// --- T5.5: TestQueryLoop_BudgetExhaustedBeforeMaxTurns ---

// TestQueryLoop_BudgetExhaustedBeforeMaxTurns는 budget 소진이 max_turns보다 먼저 발동함을 검증한다.
func TestQueryLoop_BudgetExhaustedBeforeMaxTurns(t *testing.T) {
	t.Parallel()

	// MaxTurns=5이지만 budget이 1턴에서 소진
	toolUseID := "tu_early_budget"
	toolName := "heavy_tool"
	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{
			Events:            testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
			UsageInputTokens:  60,
			UsageOutputTokens: 0, // 합계 60 > Remaining=50
		},
		// 2번째 이후는 도달하면 안 됨
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("unreachable"),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true}`, nil
	})

	cfg := makeS5Config(t, stub)
	cfg.MaxTurns = 5
	cfg.TaskBudget = query.TaskBudget{Total: 50, Remaining: 50}
	cfg.CanUseTool = testsupport.NewStubCanUseToolAllow()
	cfg.Executor = executor

	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "heavy work")
	require.NoError(t, err)
	msgs := drainMessages(out)

	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.False(t, termPayload.Success)
	assert.Equal(t, "budget_exceeded", termPayload.Error,
		"budget 먼저 소진 시 budget_exceeded가 max_turns보다 우선이어야 한다")
}

// TestQueryLoop_MaxTurnsBeforeBudget은 max_turns가 budget보다 먼저 발동함을 검증한다.
func TestQueryLoop_MaxTurnsBeforeBudget(t *testing.T) {
	t.Parallel()

	// MaxTurns=1이지만 budget은 충분
	toolUseID := "tu_maxturns_first"
	toolName := "loop_first"
	stub := testsupport.NewStubLLMCall(
		// 1턴차: tool_use → 2턴차 유발 시도하지만 MaxTurns=1 gate 발동
		testsupport.StubLLMResponse{
			Events:            testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
			UsageInputTokens:  1,
			UsageOutputTokens: 1, // 합계 2, budget 충분
		},
		// 2번째는 도달 불가
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("unreachable"),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true}`, nil
	})

	cfg := makeS5Config(t, stub)
	cfg.MaxTurns = 1
	cfg.TaskBudget = query.TaskBudget{Total: 100000, Remaining: 100000}
	cfg.CanUseTool = testsupport.NewStubCanUseToolAllow()
	cfg.Executor = executor

	engine, err := query.New(cfg)
	require.NoError(t, err)

	out, err := engine.SubmitMessage(context.Background(), "loop first")
	require.NoError(t, err)
	msgs := drainMessages(out)

	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success, "max_turns 도달은 success=true이어야 한다")
	assert.Equal(t, "max_turns", termPayload.Error,
		"budget 충분할 때 max_turns가 우선이어야 한다")
}
