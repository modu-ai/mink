// Package query tests for QueryEngine.
// SPEC-GOOSE-QUERY-001 engine_test.go
// Build tag removed to ensure tests run with standard `go test` command.
package query_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/message"
	"github.com/modu-ai/mink/internal/query"
	"github.com/modu-ai/mink/internal/query/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestQueryEngine_SubmitMessage_StreamsImmediately는 AC-QUERY-001을 검증한다.
//
// Given: StubLLMCall이 StreamEvent{delta:"ok"} + message_stop으로 응답하는 단일 assistant turn.
// When: SubmitMessage(ctx, "hi") 호출 후 채널 drain.
// Then: user_ack → stream_request_start → stream_event{delta:"ok"} → message{role:"assistant"} → terminal{success:true}
//
//	채널 close. State.TurnCount == 1.
func TestQueryEngine_SubmitMessage_StreamsImmediately(t *testing.T) {
	// Arrange
	stub := testsupport.NewStubLLMCallSimple("ok")
	executor := testsupport.NewStubExecutor()
	canUse := testsupport.NewStubCanUseToolAllow()
	logger := zaptest.NewLogger(t)

	cfg := query.QueryEngineConfig{
		LLMCall:    stub.AsFunc(),
		Tools:      []query.ToolDefinition{},
		CanUseTool: canUse,
		Executor:   executor,
		Logger:     logger,
		MaxTurns:   10,
		TaskBudget: query.TaskBudget{Total: 10000, Remaining: 10000},
	}

	engine, err := query.New(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	// Act
	out, err := engine.SubmitMessage(ctx, "hi")
	require.NoError(t, err)

	// 채널 drain
	var msgs []message.SDKMessage
	for msg := range out {
		msgs = append(msgs, msg)
	}

	// Assert: 메시지 순서 검증
	require.GreaterOrEqual(t, len(msgs), 5, "최소 5개 메시지 필요: user_ack, stream_request_start, stream_event, message, terminal")

	// 순서별 타입 검증
	assert.Equal(t, message.SDKMsgUserAck, msgs[0].Type, "첫 번째는 user_ack이어야 한다")
	assert.Equal(t, message.SDKMsgStreamRequestStart, msgs[1].Type, "두 번째는 stream_request_start이어야 한다")

	// stream_event{delta:"ok"} 검증
	var streamEventIdx int = -1
	for i, m := range msgs {
		if m.Type == message.SDKMsgStreamEvent {
			streamEventIdx = i
			break
		}
	}
	require.NotEqual(t, -1, streamEventIdx, "stream_event 메시지가 있어야 한다")
	sePayload, ok := msgs[streamEventIdx].Payload.(message.PayloadStreamEvent)
	require.True(t, ok, "stream_event payload 타입 검증")
	assert.Equal(t, "ok", sePayload.Event.Delta, "stream delta가 'ok'이어야 한다")

	// assistant message 검증
	var assistantMsgIdx int = -1
	for i, m := range msgs {
		if m.Type == message.SDKMsgMessage {
			assistantMsgIdx = i
			break
		}
	}
	require.NotEqual(t, -1, assistantMsgIdx, "assistant message가 있어야 한다")
	msgPayload, ok := msgs[assistantMsgIdx].Payload.(message.PayloadMessage)
	require.True(t, ok, "message payload 타입 검증")
	assert.Equal(t, "assistant", msgPayload.Msg.Role, "메시지 role이 assistant이어야 한다")

	// terminal 검증: 마지막 메시지
	lastMsg := msgs[len(msgs)-1]
	assert.Equal(t, message.SDKMsgTerminal, lastMsg.Type, "마지막은 terminal이어야 한다")
	termPayload, ok := lastMsg.Payload.(message.PayloadTerminal)
	require.True(t, ok, "terminal payload 타입 검증")
	assert.True(t, termPayload.Success, "terminal.success가 true이어야 한다")

	// 채널이 close되었는지는 drain 완료로 이미 검증됨 (range 종료)

	t.Run("empty_prompt", func(t *testing.T) {
		// 빈 프롬프트도 정상 처리되어야 한다.
		stubEmpty := testsupport.NewStubLLMCallSimple("ok")
		cfgEmpty := cfg
		cfgEmpty.LLMCall = stubEmpty.AsFunc()
		engEmpty, err := query.New(cfgEmpty)
		require.NoError(t, err)

		outEmpty, err := engEmpty.SubmitMessage(ctx, "")
		require.NoError(t, err)

		var emptyMsgs []message.SDKMessage
		for m := range outEmpty {
			emptyMsgs = append(emptyMsgs, m)
		}

		// terminal이 마지막에 있어야 한다.
		require.NotEmpty(t, emptyMsgs, "빈 프롬프트에도 메시지가 있어야 한다")
		lastEmpty := emptyMsgs[len(emptyMsgs)-1]
		assert.Equal(t, message.SDKMsgTerminal, lastEmpty.Type)
	})
}

// --- T9.2: TestQueryEngine_SubmitMessage_Returns10ms (AC-QUERY-014) ---

// TestQueryEngine_SubmitMessage_Returns10ms는 SubmitMessage가 LLM 초기화 지연(100ms)과 무관하게
// 10ms 이내에 반환됨을 검증한다. (AC-QUERY-014 / REQ-QUERY-016)
//
// Given: StubLLMCall에 goroutine 내부 100ms initialDelay 설정 (dial 비용 시뮬레이션).
//
//	QueryEngine.New(cfg)로 엔진 생성 완료된 상태 (초기화 비용 배제).
//
// When: N=1000 반복으로 t0 = time.Now(); SubmitMessage(...); t1 = time.Now() 측정.
// Then:
//   - 모든 t1-t0 ≤ 10ms (strict)
//   - p99 ≤ 10ms
//   - p50 ≤ 1ms
//
// @MX:NOTE: [AUTO] AC-QUERY-014 - SubmitMessage 10ms 상시 불변식. goroutine spawn으로 보장.
func TestQueryEngine_SubmitMessage_Returns10ms(t *testing.T) {
	const N = 1000
	const ceilMs = 10 * time.Millisecond
	const p50CeilMs = 1 * time.Millisecond

	// 100ms initialDelay 스텁: goroutine 내부에서 dial 비용 시뮬레이션
	stub := testsupport.NewStubLLMCall(
		testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok")},
	)
	stub.InitialDelay = int64(100 * time.Millisecond)

	cfg := query.QueryEngineConfig{
		LLMCall:    stub.AsFunc(),
		Tools:      []query.ToolDefinition{},
		CanUseTool: testsupport.NewStubCanUseToolAllow(),
		Executor:   testsupport.NewStubExecutor(),
		Logger:     zaptest.NewLogger(t),
		MaxTurns:   10,
		TaskBudget: query.TaskBudget{Total: 1_000_000, Remaining: 1_000_000},
	}

	engine, err := query.New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	samples := make([]time.Duration, 0, N)

	for i := 0; i < N; i++ {
		t0 := time.Now()
		ch, err := engine.SubmitMessage(ctx, fmt.Sprintf("hi-%d", i))
		t1 := time.Now()
		require.NoError(t, err, "SubmitMessage 에러 없어야 한다")

		elapsed := t1.Sub(t0)
		samples = append(samples, elapsed)

		// goroutine이 drain 완료될 때까지 기다린다 (다음 iteration 전에).
		// 두 번째 SubmitMessage는 첫 번째 drain 완료 후 호출되어야 한다 (직렬화 보장).
		for range ch {
		}
	}

	// 모든 샘플이 10ms 이하인지 확인
	over := 0
	for _, s := range samples {
		if s > ceilMs {
			over++
		}
	}
	assert.Equal(t, 0, over, "모든 SubmitMessage 반환이 10ms 이하이어야 한다 (초과=%d/%d)", over, N)

	// p99 계산
	p99 := percentile(samples, 0.99)
	p50 := percentile(samples, 0.50)
	t.Logf("SubmitMessage latency: p50=%v p99=%v (N=%d)", p50, p99, N)
	assert.LessOrEqual(t, p99, ceilMs, "p99 ≤ 10ms")
	assert.LessOrEqual(t, p50, p50CeilMs, "p50 ≤ 1ms")

	t.Run("fast_error", func(t *testing.T) {
		// 즉시 에러 반환하는 경우에도 SubmitMessage 자체는 10ms 이내 반환.
		errStub := testsupport.NewStubLLMCall(
			testsupport.StubLLMResponse{Err: fmt.Errorf("immediate error")},
		)
		errCfg := cfg
		errCfg.LLMCall = errStub.AsFunc()
		errEngine, err := query.New(errCfg)
		require.NoError(t, err)

		t0 := time.Now()
		ch, err := errEngine.SubmitMessage(ctx, "hi")
		t1 := time.Now()
		require.NoError(t, err)

		elapsed := t1.Sub(t0)
		assert.LessOrEqual(t, elapsed, ceilMs, "즉시 에러 시에도 SubmitMessage는 10ms 이내 반환")

		// 에러는 terminal 메시지로 전달됨
		for range ch {
		}
	})
}

// percentile은 샘플 슬라이스에서 p번째 백분위수를 반환한다.
func percentile(samples []time.Duration, p float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(samples))
	copy(sorted, samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// --- T9.4: TestQueryEngine_TeammateIdentity_InjectedEverywhere (AC-QUERY-016) ---

// TestQueryEngine_TeammateIdentity_InjectedEverywhere는 TeammateIdentity가
// (a) LLM payload system header와 (b) 모든 SDKMessage.Meta에 주입됨을 검증한다.
// (AC-QUERY-016 / REQ-QUERY-020)
//
// Given: QueryEngineConfig.TeammateIdentity = &TeammateIdentity{AgentID:"spec-ga-01", TeamName:"alpha"}
//
//	StubLLMCall payload recorder 활성.
//
// When: SubmitMessage(ctx, "hi") drain.
// Then:
//   - (a) 첫 outbound LLM call의 system 파트에 {agent_id, team_name} 포함.
//   - (b) 모든 SDKMessage의 Meta에 {agent_id, team_name} 포함.
//
// @MX:NOTE: [AUTO] AC-QUERY-016 - TeammateIdentity 두 경로 주입 검증.
func TestQueryEngine_TeammateIdentity_InjectedEverywhere(t *testing.T) {
	t.Parallel()

	identity := &query.TeammateIdentity{
		AgentID:  "spec-ga-01",
		TeamName: "alpha",
	}

	t.Run("identity_injected", func(t *testing.T) {
		t.Parallel()

		stub := testsupport.NewStubLLMCallSimple("hello")

		cfg := query.QueryEngineConfig{
			LLMCall:          stub.AsFunc(),
			Tools:            []query.ToolDefinition{},
			CanUseTool:       testsupport.NewStubCanUseToolAllow(),
			Executor:         testsupport.NewStubExecutor(),
			Logger:           zaptest.NewLogger(t),
			MaxTurns:         10,
			TaskBudget:       query.TaskBudget{Total: 10000, Remaining: 10000},
			TeammateIdentity: identity,
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out, err := engine.SubmitMessage(context.Background(), "hi")
		require.NoError(t, err)
		msgs := drainMessages(out)

		// (a) LLM payload system header 검증
		stub.RecordMu().Lock()
		requests := stub.RecordedRequests
		stub.RecordMu().Unlock()
		require.GreaterOrEqual(t, len(requests), 1, "LLM call이 최소 1회 있어야 한다")

		firstReq := requests[0]
		// SystemHeader 필드 확인 (engine이 주입한 teammate identity)
		agentID, hasAgentID := firstReq.SystemHeader["agent_id"]
		teamName, hasTeamName := firstReq.SystemHeader["team_name"]
		assert.True(t, hasAgentID, "(a) LLM payload system header에 agent_id가 있어야 한다")
		assert.True(t, hasTeamName, "(a) LLM payload system header에 team_name이 있어야 한다")
		assert.Equal(t, "spec-ga-01", agentID, "(a) agent_id 값 검증")
		assert.Equal(t, "alpha", teamName, "(a) team_name 값 검증")

		// (b) 모든 SDKMessage의 Meta에 {agent_id, team_name} 포함
		require.NotEmpty(t, msgs, "SDKMessage가 있어야 한다")
		for i, m := range msgs {
			require.NotNil(t, m.Meta, "(b) SDKMessage[%d] Meta가 nil이면 안 된다", i)
			assert.Equal(t, "spec-ga-01", m.Meta["agent_id"], "(b) SDKMessage[%d] Meta.agent_id", i)
			assert.Equal(t, "alpha", m.Meta["team_name"], "(b) SDKMessage[%d] Meta.team_name", i)
		}
	})

	t.Run("nil_identity", func(t *testing.T) {
		t.Parallel()

		stub := testsupport.NewStubLLMCallSimple("hello")

		cfg := query.QueryEngineConfig{
			LLMCall:          stub.AsFunc(),
			Tools:            []query.ToolDefinition{},
			CanUseTool:       testsupport.NewStubCanUseToolAllow(),
			Executor:         testsupport.NewStubExecutor(),
			Logger:           zaptest.NewLogger(t),
			MaxTurns:         10,
			TaskBudget:       query.TaskBudget{Total: 10000, Remaining: 10000},
			TeammateIdentity: nil,
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out, err := engine.SubmitMessage(context.Background(), "hi")
		require.NoError(t, err)
		msgs := drainMessages(out)

		// nil identity → system header 주입 없음
		stub.RecordMu().Lock()
		requests := stub.RecordedRequests
		stub.RecordMu().Unlock()
		require.GreaterOrEqual(t, len(requests), 1)

		firstReq := requests[0]
		assert.Nil(t, firstReq.SystemHeader, "nil identity → LLM system header 없음")

		// nil identity → SDKMessage Meta에 teammate 필드 없음
		for i, m := range msgs {
			if m.Meta != nil {
				assert.Nil(t, m.Meta["agent_id"], "nil identity → SDKMessage[%d] Meta에 agent_id 없음", i)
				assert.Nil(t, m.Meta["team_name"], "nil identity → SDKMessage[%d] Meta에 team_name 없음", i)
			}
		}
	})
}

// drainMessages는 SDKMessage 채널을 drain하여 슬라이스로 반환한다.
func drainMessages(out <-chan message.SDKMessage) []message.SDKMessage {
	var msgs []message.SDKMessage
	for m := range out {
		msgs = append(msgs, m)
	}
	return msgs
}
