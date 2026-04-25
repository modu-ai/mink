//go:build integration

// Package loop_test — SPEC-GOOSE-QUERY-001 S9 통합 테스트.
// T9.1: Fallback model chain (AC-QUERY-012)
// T9.3: PostSamplingHooks FIFO chain (AC-QUERY-015)
//
// 빌드 태그: integration (go test -tags=integration)
package loop_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/modu-ai/goose/internal/message"
	"github.com/modu-ai/goose/internal/query"
	"github.com/modu-ai/goose/internal/query/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// --- T9.1: TestQueryLoop_FallbackModelChain (AC-QUERY-012) ---

// TestQueryLoop_FallbackModelChain는 primary 모델 529 실패 시 fallback 모델로 투명 재시도하는
// fallback chain을 검증한다. (AC-QUERY-012 / REQ-QUERY-019)
//
// Given: QueryEngineConfig.FallbackModels = ["model-B"]
//
//	StubLLMCall: primary 호출(model=="") → error, fallback("model-B") → 정상 stop
//
// When: SubmitMessage(ctx, "please")
// Then: terminal{success:true}. 로그에 "fallback used" 필드 존재.
//
// Edge cases:
//   - FallbackModels 비어있을 때 primary 실패 → terminal{success:false, error contains "provider_overloaded"}
//   - primary 성공 시 fallback 호출 0회
func TestQueryLoop_FallbackModelChain(t *testing.T) {
	t.Parallel()

	t.Run("fallback_success", func(t *testing.T) {
		t.Parallel()

		// primary 호출이 항상 실패하는 stub, fallback 호출은 성공.
		// StubLLMCallWithFallback은 req.Route.Model 기반으로 다른 시퀀스를 반환한다.
		stub := testsupport.NewStubLLMCallWithFallback(
			nil, // primary: error
			[]testsupport.StubLLMResponse{
				{
					Events: testsupport.MakeStopEvents("fallback response"),
				},
			},
		)

		core, logs := observer.New(zapcore.DebugLevel)
		logger := zap.New(core)

		cfg := query.QueryEngineConfig{
			LLMCall:        stub.AsFunc(),
			Tools:          []query.ToolDefinition{},
			CanUseTool:     testsupport.NewStubCanUseToolAllow(),
			Executor:       testsupport.NewStubExecutor(),
			Logger:         logger,
			MaxTurns:       10,
			TaskBudget:     query.TaskBudget{Total: 10000, Remaining: 10000},
			FallbackModels: []string{"model-B"},
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out, err := engine.SubmitMessage(context.Background(), "please")
		require.NoError(t, err)
		msgs := drainMessages(out)

		// terminal{success:true} 확인
		last := msgs[len(msgs)-1]
		require.Equal(t, message.SDKMsgTerminal, last.Type)
		term, ok := last.Payload.(message.PayloadTerminal)
		require.True(t, ok)
		assert.True(t, term.Success, "fallback 성공 시 terminal.success=true")

		// 로그에 "fallback used" 필드 확인
		found := false
		for _, entry := range logs.All() {
			if entry.Message == "fallback used" || containsField(entry.Context, "fallback_model") {
				found = true
				break
			}
		}
		assert.True(t, found, "로그에 fallback 사용 기록이 있어야 한다")
	})

	t.Run("no_fallback_models_primary_fails", func(t *testing.T) {
		t.Parallel()

		// FallbackModels 비어있고 primary 실패 → terminal{success:false}
		stub := testsupport.NewStubLLMCallWithFallback(nil, nil)

		cfg := query.QueryEngineConfig{
			LLMCall:        stub.AsFunc(),
			Tools:          []query.ToolDefinition{},
			CanUseTool:     testsupport.NewStubCanUseToolAllow(),
			Executor:       testsupport.NewStubExecutor(),
			Logger:         makeLoopConfig(t, stub.Primary(), testsupport.NewStubCanUseToolAllow(), testsupport.NewStubExecutor()).Logger,
			MaxTurns:       10,
			TaskBudget:     query.TaskBudget{Total: 10000, Remaining: 10000},
			FallbackModels: nil,
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out, err := engine.SubmitMessage(context.Background(), "please")
		require.NoError(t, err)
		msgs := drainMessages(out)

		last := msgs[len(msgs)-1]
		require.Equal(t, message.SDKMsgTerminal, last.Type)
		term, ok := last.Payload.(message.PayloadTerminal)
		require.True(t, ok)
		assert.False(t, term.Success, "FallbackModels 없이 primary 실패 시 terminal.success=false")
	})

	t.Run("primary_success_no_fallback_call", func(t *testing.T) {
		t.Parallel()

		// primary 성공 → fallback 호출 0회
		stub := testsupport.NewStubLLMCallWithFallback(
			[]testsupport.StubLLMResponse{
				{Events: testsupport.MakeStopEvents("ok")},
			},
			nil, // fallback이 호출되면 안 됨
		)

		cfg := query.QueryEngineConfig{
			LLMCall:        stub.AsFunc(),
			Tools:          []query.ToolDefinition{},
			CanUseTool:     testsupport.NewStubCanUseToolAllow(),
			Executor:       testsupport.NewStubExecutor(),
			Logger:         makeLoopConfig(t, stub.Primary(), testsupport.NewStubCanUseToolAllow(), testsupport.NewStubExecutor()).Logger,
			MaxTurns:       10,
			TaskBudget:     query.TaskBudget{Total: 10000, Remaining: 10000},
			FallbackModels: []string{"model-B"},
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out, err := engine.SubmitMessage(context.Background(), "please")
		require.NoError(t, err)
		msgs := drainMessages(out)

		last := msgs[len(msgs)-1]
		require.Equal(t, message.SDKMsgTerminal, last.Type)
		term, ok := last.Payload.(message.PayloadTerminal)
		require.True(t, ok)
		assert.True(t, term.Success, "primary 성공 시 terminal.success=true")
		assert.Equal(t, 0, stub.FallbackCallCount(), "primary 성공 시 fallback 호출 0회")
	})
}

// containsField는 zapcore.Field 슬라이스에서 지정한 키를 가진 필드가 있는지 확인한다.
func containsField(fields []zapcore.Field, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

// --- T9.3: TestQueryLoop_PostSamplingHooks_FifoChain (AC-QUERY-015) ---

// TestQueryLoop_PostSamplingHooks_FifoChain는 PostSamplingHooks가 FIFO 순으로 적용되는지
// 검증한다. (AC-QUERY-015 / REQ-QUERY-018)
//
// Given: PostSamplingHooks = [h1, h2]
//
//	h1: content 말미에 " [h1]" 추가
//	h2: content 말미에 " [h2]" 추가
//	stub LLM = "ok"
//
// When: SubmitMessage("hi") drain
// Then: 2번째 LLM call payload의 messages[] 중 assistant.content == "ok [h1] [h2]"
//
//	또는 State.Messages의 assistant content == "ok [h1] [h2]"
func TestQueryLoop_PostSamplingHooks_FifoChain(t *testing.T) {
	t.Parallel()

	makeHook := func(suffix string) query.MessageHook {
		return func(_ context.Context, msg message.Message) (message.Message, error) {
			for i, block := range msg.Content {
				if block.Type == "text" {
					msg.Content[i].Text += suffix
				}
			}
			return msg, nil
		}
	}

	t.Run("fifo_h1_then_h2", func(t *testing.T) {
		t.Parallel()

		h1 := makeHook(" [h1]")
		h2 := makeHook(" [h2]")

		// 2번의 LLM call을 기록하는 stub:
		// 1st call: "ok" → 2nd call: "ok3" (2번째 SubmitMessage에서 호출됨)
		stub := testsupport.NewStubLLMCall(
			testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok")},
			testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok3")},
		)

		cfg := query.QueryEngineConfig{
			LLMCall:           stub.AsFunc(),
			Tools:             []query.ToolDefinition{},
			CanUseTool:        testsupport.NewStubCanUseToolAllow(),
			Executor:          testsupport.NewStubExecutor(),
			Logger:            makeLoopConfig(t, testsupport.NewStubLLMCallSimple("x"), testsupport.NewStubCanUseToolAllow(), testsupport.NewStubExecutor()).Logger,
			MaxTurns:          10,
			TaskBudget:        query.TaskBudget{Total: 10000, Remaining: 10000},
			PostSamplingHooks: []query.MessageHook{h1, h2},
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		// 1번째 SubmitMessage drain
		out1, err := engine.SubmitMessage(context.Background(), "hi")
		require.NoError(t, err)
		drainMessages(out1)

		// 2번째 SubmitMessage: payload에 이전 assistant content가 포함됨
		out2, err := engine.SubmitMessage(context.Background(), "world")
		require.NoError(t, err)
		drainMessages(out2)

		// 2번째 LLM call의 messages에 첫 번째 assistant content가 hook 적용되어야 함
		stub.RecordMu().Lock()
		requests := stub.RecordedRequests
		stub.RecordMu().Unlock()
		require.GreaterOrEqual(t, len(requests), 2, "2번째 LLM call이 있어야 한다")
		secondCallMsgs := requests[1].Messages

		// assistant role의 messages에서 hook 적용 여부 확인
		found := false
		for _, msg := range secondCallMsgs {
			if msg.Role == "assistant" {
				for _, block := range msg.Content {
					if block.Type == "text" && block.Text == "ok [h1] [h2]" {
						found = true
					}
				}
			}
		}
		assert.True(t, found, "2번째 LLM call의 messages에 hook 적용된 'ok [h1] [h2]'가 있어야 한다")
	})

	t.Run("fifo_h2_then_h1", func(t *testing.T) {
		t.Parallel()

		h1 := makeHook(" [h1]")
		h2 := makeHook(" [h2]")

		stub := testsupport.NewStubLLMCall(
			testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok")},
			testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok3")},
		)

		cfg := query.QueryEngineConfig{
			LLMCall:           stub.AsFunc(),
			Tools:             []query.ToolDefinition{},
			CanUseTool:        testsupport.NewStubCanUseToolAllow(),
			Executor:          testsupport.NewStubExecutor(),
			Logger:            makeLoopConfig(t, testsupport.NewStubLLMCallSimple("x"), testsupport.NewStubCanUseToolAllow(), testsupport.NewStubExecutor()).Logger,
			MaxTurns:          10,
			TaskBudget:        query.TaskBudget{Total: 10000, Remaining: 10000},
			PostSamplingHooks: []query.MessageHook{h2, h1}, // 순서 바꿈
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out1, err := engine.SubmitMessage(context.Background(), "hi")
		require.NoError(t, err)
		drainMessages(out1)

		out2, err := engine.SubmitMessage(context.Background(), "world")
		require.NoError(t, err)
		drainMessages(out2)

		// 2번째 call에서 assistant content가 "ok [h2] [h1]"이어야 함
		requests := stub.RecordedRequests
		require.GreaterOrEqual(t, len(requests), 2, "2번째 LLM call이 있어야 한다")
		secondCallMsgs := requests[1].Messages

		found := false
		for _, msg := range secondCallMsgs {
			if msg.Role == "assistant" {
				for _, block := range msg.Content {
					if block.Type == "text" && block.Text == "ok [h2] [h1]" {
						found = true
					}
				}
			}
		}
		assert.True(t, found, "순서 변경 시 messages에 hook 적용된 'ok [h2] [h1]'가 있어야 한다")
	})

	t.Run("no_hooks_no_modification", func(t *testing.T) {
		t.Parallel()

		stub := testsupport.NewStubLLMCall(
			testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok")},
			testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok3")},
		)

		cfg := query.QueryEngineConfig{
			LLMCall:           stub.AsFunc(),
			Tools:             []query.ToolDefinition{},
			CanUseTool:        testsupport.NewStubCanUseToolAllow(),
			Executor:          testsupport.NewStubExecutor(),
			Logger:            makeLoopConfig(t, testsupport.NewStubLLMCallSimple("x"), testsupport.NewStubCanUseToolAllow(), testsupport.NewStubExecutor()).Logger,
			MaxTurns:          10,
			TaskBudget:        query.TaskBudget{Total: 10000, Remaining: 10000},
			PostSamplingHooks: nil, // 훅 없음
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out1, err := engine.SubmitMessage(context.Background(), "hi")
		require.NoError(t, err)
		drainMessages(out1)

		out2, err := engine.SubmitMessage(context.Background(), "world")
		require.NoError(t, err)
		drainMessages(out2)

		requests := stub.RecordedRequests
		require.GreaterOrEqual(t, len(requests), 2)
		secondCallMsgs := requests[1].Messages

		// 훅 없으면 content 변형 없어야 함
		for _, msg := range secondCallMsgs {
			if msg.Role == "assistant" {
				for _, block := range msg.Content {
					if block.Type == "text" {
						assert.Equal(t, "ok", block.Text, "훅 없으면 content 변형 없어야 한다")
					}
				}
			}
		}
	})

	t.Run("hook_error_stops_chain", func(t *testing.T) {
		t.Parallel()

		// h1은 에러 반환, h2는 정상이지만 h1 에러 후 체인이 중단되어야 함
		var h2Called bool
		h1Err := func(_ context.Context, msg message.Message) (message.Message, error) {
			return msg, fmt.Errorf("hook error")
		}
		h2 := func(_ context.Context, msg message.Message) (message.Message, error) {
			h2Called = true
			for i, block := range msg.Content {
				if block.Type == "text" {
					msg.Content[i].Text += " [h2]"
				}
			}
			return msg, nil
		}

		stub := testsupport.NewStubLLMCall(
			testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok")},
			testsupport.StubLLMResponse{Events: testsupport.MakeStopEvents("ok3")},
		)

		cfg := query.QueryEngineConfig{
			LLMCall:           stub.AsFunc(),
			Tools:             []query.ToolDefinition{},
			CanUseTool:        testsupport.NewStubCanUseToolAllow(),
			Executor:          testsupport.NewStubExecutor(),
			Logger:            makeLoopConfig(t, testsupport.NewStubLLMCallSimple("x"), testsupport.NewStubCanUseToolAllow(), testsupport.NewStubExecutor()).Logger,
			MaxTurns:          10,
			TaskBudget:        query.TaskBudget{Total: 10000, Remaining: 10000},
			PostSamplingHooks: []query.MessageHook{h1Err, h2},
		}

		engine, err := query.New(cfg)
		require.NoError(t, err)

		out1, err := engine.SubmitMessage(context.Background(), "hi")
		require.NoError(t, err)
		drainMessages(out1)

		out2, err := engine.SubmitMessage(context.Background(), "world")
		require.NoError(t, err)
		drainMessages(out2)

		// h1이 에러를 반환하면 h2는 호출되지 않아야 한다
		assert.False(t, h2Called, "h1 에러 후 h2가 호출되면 안 된다")
	})
}
