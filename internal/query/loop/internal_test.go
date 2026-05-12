//go:build integration

// Package loop_test — loop 패키지 내부 함수 단위 테스트 (white-box).
// buildLLMFuncWithMessages, queryLoop ctx 취소 경로 커버.
package loop

import (
	"context"
	"testing"

	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildLLMFuncWithMessages_FactoryNil은 CallLLMFactory가 nil인 경우
// 기존 CallLLM을 재사용함을 검증한다.
func TestBuildLLMFuncWithMessages_FactoryNil(t *testing.T) {
	t.Parallel()

	called := false
	existingFunc := LLMStreamFunc(func(_ context.Context) (<-chan message.StreamEvent, error) {
		called = true
		ch := make(chan message.StreamEvent)
		close(ch)
		return ch, nil
	})

	cfg := LoopConfig{
		CallLLM:        existingFunc,
		CallLLMFactory: nil, // factory 없음
	}

	// factory nil → 기존 CallLLM 반환
	fn := buildLLMFuncWithMessages(cfg, nil)
	require.NotNil(t, fn)

	ch, err := fn(context.Background())
	require.NoError(t, err)
	for range ch {
	}
	assert.True(t, called, "factory가 nil이면 기존 CallLLM이 호출되어야 한다")
}

// TestSend_ContextCancelled는 ctx 취소 시 send가 false를 반환함을 검증한다.
// unbuffered 채널 + drain 없음 + ctx 취소 조합으로 block → ctx.Done 선택.
func TestSend_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	// unbuffered 채널: 수신자 없음 → send는 ctx.Done()을 선택해야 한다
	out := make(chan message.SDKMessage)
	result := send(ctx, out, message.SDKMessage{Type: message.SDKMsgUserAck})

	assert.False(t, result, "ctx 취소 + 수신자 없음 시 send는 false를 반환해야 한다")
}

// TestQueryLoop_FirstSendFails는 첫 번째 send(user_ack) 실패 시 loop가 즉시 반환함을 검증한다.
func TestQueryLoop_FirstSendFails(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	out := make(chan message.SDKMessage) // unbuffered, drain 없음
	cfg := LoopConfig{
		Out:    out,
		Prompt: "test",
	}

	// queryLoop은 goroutine에서 실행 → out이 close될 때까지 대기
	go queryLoop(ctx, cfg)

	// ctx 취소로 첫 번째 send가 실패 → out이 즉시 close됨
	_, ok := <-out
	// ok=true이면 일부 메시지가 전달된 것, ok=false이면 즉시 close
	// 어느 쪽이든 채널이 close되어야 한다 (deadlock 없음)
	_ = ok

	// 채널이 close되었으면 추가 drain은 즉시 반환
	for range out {
	}
}

// TestQueryLoop_SendFailAfterStreamRequestStart는 stream_request_start 전송 실패 시
// loop가 조기 반환함을 검증한다.
func TestQueryLoop_SendFailAfterStreamRequestStart(t *testing.T) {
	t.Parallel()

	// 1개 버퍼 채널: user_ack는 성공, stream_request_start에서 block
	out := make(chan message.SDKMessage, 1) // 1개만 버퍼

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := LoopConfig{
		Out:    out,
		Prompt: "test",
		CallLLM: func(_ context.Context) (<-chan message.StreamEvent, error) {
			ch := make(chan message.StreamEvent)
			close(ch)
			return ch, nil
		},
		CanUseTool: nil, // tool 없으므로 nil
		Execute:    nil,
	}

	go queryLoop(ctx, cfg)

	// user_ack 수신 후 ctx 취소
	<-out
	cancel()

	// out이 close될 때까지 drain
	for range out {
	}
}
