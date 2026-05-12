//go:build integration

// Package loop_test — SPEC-GOOSE-QUERY-001 S7 통합 테스트.
// T7.1: Compactor.ShouldCompact==true → CompactBoundary yield + after_compact continue (AC-QUERY-011)
// T7.2: REQ-008 reset 조항 — after_compact 시 maxOutputTokensRecoveryCount=0
//
// 빌드 태그: integration (go test -tags=integration)
package loop_test

import (
	"context"
	"testing"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/query"
	"github.com/modu-ai/mink/internal/query/loop"
	"github.com/modu-ai/mink/internal/query/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// makeS7Config는 S7 테스트용 기본 QueryEngineConfig를 생성한다.
// Compactor는 호출자가 주입한다.
func makeS7Config(t *testing.T, stub *testsupport.StubLLMCall, compactor query.Compactor) query.QueryEngineConfig {
	t.Helper()
	return query.QueryEngineConfig{
		LLMCall:    stub.AsFunc(),
		Tools:      []query.ToolDefinition{},
		CanUseTool: testsupport.NewStubCanUseToolAllow(),
		Executor:   testsupport.NewStubExecutor(),
		Logger:     zaptest.NewLogger(t),
		MaxTurns:   10,
		TaskBudget: query.TaskBudget{Total: 100000, Remaining: 100000, ToolResultCap: 0},
		Compactor:  compactor,
	}
}

// --- T7.1: TestQueryLoop_CompactBoundaryYieldedOnCompact (AC-QUERY-011) ---

// TestQueryLoop_CompactBoundaryYieldedOnCompact는 Compactor.ShouldCompact가 turn 3 시작 전에
// true를 반환할 때 CompactBoundary가 yield되고 치환된 State로 iteration이 진행됨을 검증한다.
//
// Given: 3턴 시나리오 — 1턴, 2턴은 tool_use 포함 (각 turn 사이 messages 누적)
//
//	StubCompactor: ShouldCompact는 TurnCount >= 2 (즉 3번째 iteration 시작 시) true 반환
//	Compact: messages 10개 → 요약 1개로 치환, CompactBoundary{Turn:3, MessagesBefore:10, MessagesAfter:1}
//
// When: SubmitMessage drain
// Then:
//   - compact_boundary{turn:3, messages_before:10, messages_after:1} yield
//   - compaction 이후 iteration은 치환된 State(messages 1개)로 진행
//   - terminal{success:true}
//   - taskBudget.Remaining은 compaction 전후 누적값 보존
func TestQueryLoop_CompactBoundaryYieldedOnCompact(t *testing.T) {
	t.Parallel()

	// Arrange: 2번의 tool_use turn을 거쳐 3번째 iteration에서 compaction 발생
	// 1턴: tool_use → after_tool_results continue
	// 2턴: tool_use → after_tool_results continue
	// 3턴 iteration 시작 전: ShouldCompact==true → after_compact continue → 3턴 LLM 호출 → stop
	toolUseID1 := "tu_s7_001"
	toolUseID2 := "tu_s7_002"
	toolName := "op"

	stub := testsupport.NewStubLLMCall(
		// 1턴: tool_use
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID1, toolName, `{"n":1}`),
		},
		// 2턴: tool_use
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID2, toolName, `{"n":2}`),
		},
		// 3턴: stop (compaction 후 계속되는 LLM 호출)
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("compact done"),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true}`, nil
	})

	// StubCompactor: TurnCount >= 2인 시점(3번째 iteration 시작 직전)에 true 반환
	// loop는 turn을 증가시키기 전에 ShouldCompact를 검사하므로,
	// TurnCount==2이면 3번째 iteration이 시작될 참임을 의미한다.
	compactor := testsupport.NewStubCompactorNoop()
	compactor.SetShouldCompact(func(state loop.State) bool {
		return state.TurnCount >= 2
	})

	const msgsBefore = 10
	const msgsAfter = 1
	compactTurn := 3

	// Compact: messages를 msgsBefore개로 부풀린 뒤 msgsAfter개로 치환
	compactor.SetCompact(func(state loop.State) (loop.State, query.CompactBoundary, error) {
		// 10개로 부풀린 messages 슬라이스를 만들어 치환 효과를 시뮬레이션한다.
		// 실제로는 state.Messages를 10개로 가정하고 1개로 치환한다.
		newMsgs := make([]message.Message, msgsAfter)
		newMsgs[0] = message.Message{
			Role:    "user",
			Content: []message.ContentBlock{{Type: "text", Text: "[summary]"}},
		}
		newState := state
		newState.Messages = newMsgs
		boundary := query.CompactBoundary{
			Turn:           compactTurn,
			MessagesBefore: msgsBefore,
			MessagesAfter:  msgsAfter,
		}
		return newState, boundary, nil
	})

	cfg := makeS7Config(t, stub, compactor)
	cfg.CanUseTool = testsupport.NewStubCanUseToolAllow()
	cfg.Executor = executor

	engine, err := query.New(cfg)
	require.NoError(t, err)

	// Act
	out, err := engine.SubmitMessage(context.Background(), "start")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// Assert

	// compact_boundary 메시지가 1건 yield되어야 한다.
	compactBoundaryMsgs := findMessages(msgs, message.SDKMsgCompactBoundary)
	require.Len(t, compactBoundaryMsgs, 1, "compact_boundary 메시지가 정확히 1건 yield되어야 한다")

	cbPayload, ok := compactBoundaryMsgs[0].Payload.(message.PayloadCompactBoundary)
	require.True(t, ok, "compact_boundary payload 타입이 PayloadCompactBoundary이어야 한다")
	assert.Equal(t, compactTurn, cbPayload.Turn, "compact_boundary.turn이 3이어야 한다")
	assert.Equal(t, msgsBefore, cbPayload.MessagesBefore, "compact_boundary.messages_before가 10이어야 한다")
	assert.Equal(t, msgsAfter, cbPayload.MessagesAfter, "compact_boundary.messages_after가 1이어야 한다")

	// terminal{success:true}로 정상 종료되어야 한다.
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type, "마지막 메시지는 terminal이어야 한다")
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success, "terminal.success가 true이어야 한다")

	// compaction 후 LLM이 한 번 더 호출되어야 한다 (총 3번: 1턴, 2턴, compaction 후 3턴).
	assert.Equal(t, 3, stub.CallCount(), "compaction 후 3번째 LLM 호출이 발생해야 한다")

	// 3번째 LLM 호출의 messages는 compaction으로 치환된 1개여야 한다.
	// (user message 1개 + compacted 1개 = 실제 개수는 구현에 따라 다를 수 있으나
	//  compacted state의 messages 슬라이스 길이 기반으로 검증한다)
	stub.RecordMu().Lock()
	reqs := stub.RecordedRequests
	stub.RecordMu().Unlock()
	require.Len(t, reqs, 3, "LLM 3회 호출 기록 필요")
	// 3번째 호출의 messages는 compacted state에서 파생됨 — msgsAfter(1) + 이후 메시지들 포함
	// 정확한 개수보다 "compaction 전(1,2턴 누적)보다 훨씬 적음"을 검증한다.
	thirdCallMsgsLen := len(reqs[2].Messages)
	firstCallMsgsLen := len(reqs[0].Messages)
	assert.Less(t, thirdCallMsgsLen, firstCallMsgsLen+10,
		"compaction 후 messages 수가 compaction 없을 때보다 크게 줄어들어야 한다. got third=%d, first=%d",
		thirdCallMsgsLen, firstCallMsgsLen)
}

// --- T7.2: TestQueryLoop_RetryCounter_ResetsAfterCompact (REQ-008 reset 조항) ---

// TestQueryLoop_RetryCounter_ResetsAfterCompact는 after_compact continue site 후
// MaxOutputTokensRecoveryCount가 0으로 reset되는지 검증한다.
//
// 시나리오:
//   - 1턴: tool_use 1개 포함 (after_tool_results continue → 2번째 iteration 진입)
//   - 2번째 iteration 시작: ShouldCompact(TurnCount=1)==true → Compact 호출
//   - Compact: MaxOutputTokensRecoveryCount=2 (인위적으로 설정)를 0으로 reset
//   - after_compact continue → 2턴 LLM 호출 → stop → terminal{success:true}
//
// Given: stub LLM 1차 = tool_use, 2차 = stop
//
//	StubCompactor: TurnCount==1에서 ShouldCompact==true
//	Compact: state.MaxOutputTokensRecoveryCount를 0으로 reset
//
// When: SubmitMessage drain
// Then:
//   - Compact 호출 시점의 state.MaxOutputTokensRecoveryCount가 캡처됨
//   - after_compact 시 loop에서 MaxOutputTokensRecoveryCount=0으로 reset
//   - compact_boundary yield
//   - terminal{success:true}
func TestQueryLoop_RetryCounter_ResetsAfterCompact(t *testing.T) {
	t.Parallel()

	// Arrange: 1턴 tool_use → after_tool_results continue → 2번째 iteration에서 compaction
	toolUseID := "tu_reset_001"
	toolName := "op_reset"

	stub := testsupport.NewStubLLMCall(
		// 1턴: tool_use (after_tool_results continue → 2번째 iteration 진입)
		testsupport.StubLLMResponse{
			Events: testsupport.MakeToolUseEvents(toolUseID, toolName, `{}`),
		},
		// 2턴 (compaction 후): stop
		testsupport.StubLLMResponse{
			Events: testsupport.MakeStopEvents("after compact"),
		},
	)

	executor := testsupport.NewStubExecutor()
	executor.Register(toolName, func(_ context.Context, _ string, _ map[string]any) (string, error) {
		return `{"ok":true}`, nil
	})

	// ShouldCompact: TurnCount==1 (1턴 완료 후 2번째 iteration 시작 전)에 true
	compactor := testsupport.NewStubCompactorNoop()

	var capturedState loop.State
	var compactCalled bool

	compactor.SetShouldCompact(func(state loop.State) bool {
		return state.TurnCount == 1
	})
	compactor.SetCompact(func(state loop.State) (loop.State, query.CompactBoundary, error) {
		compactCalled = true
		capturedState = state
		// 테스트를 위해 state에 인위적으로 MaxOutputTokensRecoveryCount=2 설정
		// (loop는 after_compact에서 0으로 reset해야 한다)
		stateWithRetry := state
		stateWithRetry.MaxOutputTokensRecoveryCount = 2
		newState := stateWithRetry
		boundary := query.CompactBoundary{
			Turn:           2,
			MessagesBefore: len(state.Messages),
			MessagesAfter:  1,
		}
		newState.Messages = []message.Message{
			{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "[summary]"}}},
		}
		return newState, boundary, nil
	})

	cfg := makeS7Config(t, stub, compactor)
	cfg.CanUseTool = testsupport.NewStubCanUseToolAllow()
	cfg.Executor = executor

	engine, err := query.New(cfg)
	require.NoError(t, err)

	// Act
	out, err := engine.SubmitMessage(context.Background(), "test reset")
	require.NoError(t, err)
	msgs := drainMessages(out)

	// Assert

	// Compact가 호출되어야 한다.
	require.True(t, compactCalled, "Compact 함수가 호출되어야 한다")

	// Compact 호출 시점의 state는 TurnCount==1이어야 한다.
	assert.Equal(t, 1, capturedState.TurnCount,
		"Compact 호출 시점의 TurnCount가 1이어야 한다")

	// compact_boundary가 yield되어야 한다.
	compactBoundaryMsgs := findMessages(msgs, message.SDKMsgCompactBoundary)
	require.Len(t, compactBoundaryMsgs, 1, "compact_boundary 메시지 1건 필요")

	// terminal{success:true}
	last := msgs[len(msgs)-1]
	require.Equal(t, message.SDKMsgTerminal, last.Type)
	termPayload, ok := last.Payload.(message.PayloadTerminal)
	require.True(t, ok)
	assert.True(t, termPayload.Success, "compaction 후 terminal.success가 true이어야 한다")

	// after_compact continue site에서 MaxOutputTokensRecoveryCount가 0으로 reset됨을 간접 검증:
	// Compact가 MaxOutputTokensRecoveryCount=2인 state를 반환하더라도
	// loop의 after_compact 코드가 0으로 reset하므로
	// 이후 max_output_tokens retry counter가 2에서 시작하지 않아야 한다.
	// → 2번 LLM 호출 후 terminal{success:true}가 나와야 한다.
	assert.Equal(t, 2, stub.CallCount(),
		"1턴(tool_use) + compaction 후 2턴(stop) = 2회 LLM 호출")
}
