//go:build integration

// Package query integration tests.
// SPEC-GOOSE-QUERY-001 engine_test.go
// 빌드 태그: integration (go test -tags=integration)
package query_test

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
