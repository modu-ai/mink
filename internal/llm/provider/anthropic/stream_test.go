package anthropic_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/provider/anthropic"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// sampleSSE는 Anthropic SSE 응답 샘플이다.
const sampleSSEHappyPath = `event: message_start
data: {"type":"message_start","message":{"id":"msg-123","role":"assistant"}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" World"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}

event: message_stop
data: {"type":"message_stop"}

`

// sampleSSEWithToolUse는 tool_use SSE 응답 샘플이다.
const sampleSSEWithToolUse = `event: message_start
data: {"type":"message_start","message":{"id":"msg-456","role":"assistant"}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"tu-789","name":"get_weather"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"location\":"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"Seoul\"}"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_stop
data: {"type":"message_stop"}

`

// drainStream은 채널에서 모든 이벤트를 수집한다.
func drainStream(t *testing.T, ctx context.Context, ch <-chan message.StreamEvent) []message.StreamEvent {
	t.Helper()
	var events []message.StreamEvent
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, evt)
		case <-ctx.Done():
			t.Error("채널 drain 타임아웃")
			return events
		}
	}
}

// TestParseAndConvert_HappyPath는 정상 SSE 파싱과 StreamEvent 변환을 검증한다.
func TestParseAndConvert_HappyPath(t *testing.T) {
	t.Parallel()

	logger, _ := zap.NewDevelopment()
	body := io.NopCloser(strings.NewReader(sampleSSEHappyPath))

	out := make(chan message.StreamEvent, 16)
	ctx := context.Background()

	go anthropic.ParseAndConvert(ctx, body, out, logger)

	events := drainStream(t, ctx, out)

	// 최소 이벤트 검증
	require.NotEmpty(t, events)

	// message_start 확인
	assert.Equal(t, message.TypeMessageStart, events[0].Type)

	// text_delta 확인
	var textDeltas []string
	for _, evt := range events {
		if evt.Type == message.TypeTextDelta {
			textDeltas = append(textDeltas, evt.Delta)
		}
	}
	assert.Equal(t, []string{"Hello", " World"}, textDeltas)

	// message_stop 확인
	lastType := events[len(events)-1].Type
	assert.Equal(t, message.TypeMessageStop, lastType)
}

// TestParseAndConvert_ToolUse는 tool_use SSE 파싱을 검증한다 (AC-ADAPTER-002).
func TestParseAndConvert_ToolUse(t *testing.T) {
	t.Parallel()

	logger, _ := zap.NewDevelopment()
	body := io.NopCloser(strings.NewReader(sampleSSEWithToolUse))

	out := make(chan message.StreamEvent, 16)
	ctx := context.Background()

	go anthropic.ParseAndConvert(ctx, body, out, logger)

	events := drainStream(t, ctx, out)

	require.NotEmpty(t, events)

	// content_block_start{tool_use} 에서 ToolUseID 추출 검증
	var toolStartEvent *message.StreamEvent
	for i := range events {
		if events[i].Type == message.TypeContentBlockStart && events[i].BlockType == "tool_use" {
			toolStartEvent = &events[i]
			break
		}
	}
	require.NotNil(t, toolStartEvent, "content_block_start{tool_use} 이벤트 없음")
	assert.Equal(t, "tu-789", toolStartEvent.ToolUseID)

	// input_json_delta 확인
	var jsonDeltas []string
	for _, evt := range events {
		if evt.Type == message.TypeInputJSONDelta {
			jsonDeltas = append(jsonDeltas, evt.Delta)
		}
	}
	assert.Len(t, jsonDeltas, 2)
}

// TestParseAndConvert_ContextCancel은 ctx 취소 시 채널이 닫히는지 검증한다.
func TestParseAndConvert_ContextCancel(t *testing.T) {
	t.Parallel()

	logger, _ := zap.NewDevelopment()

	// 무한 대기하는 reader (실제로는 블로킹되지 않게 pr/pw 사용)
	pr, pw := io.Pipe()
	body := io.NopCloser(pr)

	out := make(chan message.StreamEvent, 8)
	ctx, cancel := context.WithCancel(context.Background())

	go anthropic.ParseAndConvert(ctx, body, out, logger)

	// 즉시 취소
	cancel()
	pw.Close()

	// 채널이 닫혔는지 확인 (goroutine이 종료되어야 함)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()

	closed := false
	for {
		select {
		case _, ok := <-out:
			if !ok {
				closed = true
				goto done
			}
		case <-ctx2.Done():
			goto done
		}
	}
done:
	assert.True(t, closed, "ctx 취소 후 채널이 닫혀야 함")
}
