package openai

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// makeBody는 SSE 라인들을 io.ReadCloser로 변환한다.
func makeBody(lines []string) io.ReadCloser {
	return io.NopCloser(strings.NewReader(strings.Join(lines, "\n") + "\n"))
}

// TestStream_TextOnly는 텍스트만 있는 스트림을 테스트한다.
func TestStream_TextOnly(t *testing.T) {
	t.Parallel()
	events := []string{
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":" world"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}
	body := makeBody(events)
	out := make(chan message.StreamEvent, 32)
	ctx := context.Background()

	go ParseAndConvert(ctx, body, out, 60*time.Second)

	var evts []message.StreamEvent
	for e := range out {
		evts = append(evts, e)
	}

	// text_delta 2개 + message_stop 1개
	textDeltas := filterByType(evts, message.TypeTextDelta)
	require.Len(t, textDeltas, 2)
	assert.Equal(t, "Hello", textDeltas[0].Delta)
	assert.Equal(t, " world", textDeltas[1].Delta)

	stops := filterByType(evts, message.TypeMessageStop)
	require.Len(t, stops, 1)
}

// TestStream_ToolCall_Fragmented는 tool_call arguments가 여러 청크로 나뉜 경우를 테스트한다.
func TestStream_ToolCall_Fragmented(t *testing.T) {
	t.Parallel()
	events := []string{
		// tool_call 시작 (name 포함)
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
		``,
		// arguments 첫 번째 조각
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\""}}]},"finish_reason":null}]}`,
		``,
		// arguments 두 번째 조각
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\":\"Seoul\"}"}}]},"finish_reason":null}]}`,
		``,
		// finish_reason=tool_calls
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		``,
		`data: [DONE]`,
		``,
	}
	body := makeBody(events)
	out := make(chan message.StreamEvent, 32)
	ctx := context.Background()

	go ParseAndConvert(ctx, body, out, 60*time.Second)

	var evts []message.StreamEvent
	for e := range out {
		evts = append(evts, e)
	}

	// content_block_start (tool_use 블록 시작) 검증
	blockStarts := filterByType(evts, message.TypeContentBlockStart)
	require.Len(t, blockStarts, 1)
	assert.Equal(t, "tool_use", blockStarts[0].BlockType)
	assert.Equal(t, "call_abc", blockStarts[0].ToolUseID)

	// input_json_delta 2개 검증
	jsonDeltas := filterByType(evts, message.TypeInputJSONDelta)
	require.Len(t, jsonDeltas, 2)
	assert.Equal(t, `{"city"`, jsonDeltas[0].Delta)
	assert.Equal(t, `":"Seoul"}`, jsonDeltas[1].Delta)

	// content_block_stop (tool_use 블록 종료) 검증
	blockStops := filterByType(evts, message.TypeContentBlockStop)
	require.Len(t, blockStops, 1)

	// message_stop 검증
	stops := filterByType(evts, message.TypeMessageStop)
	require.Len(t, stops, 1)
}

// TestStream_Mixed는 텍스트와 tool_call이 혼합된 경우를 테스트한다.
func TestStream_Mixed(t *testing.T) {
	t.Parallel()
	events := []string{
		// 텍스트 먼저
		`data: {"id":"chatcmpl-3","choices":[{"index":0,"delta":{"role":"assistant","content":"Let me check"},"finish_reason":null}]}`,
		``,
		// tool_call
		`data: {"id":"chatcmpl-3","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_xyz","type":"function","function":{"name":"search","arguments":""}}]},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-3","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"q\":\"test\"}"}}]},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-3","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		``,
		`data: [DONE]`,
		``,
	}
	body := makeBody(events)
	out := make(chan message.StreamEvent, 32)
	ctx := context.Background()

	go ParseAndConvert(ctx, body, out, 60*time.Second)

	var evts []message.StreamEvent
	for e := range out {
		evts = append(evts, e)
	}

	textDeltas := filterByType(evts, message.TypeTextDelta)
	require.Len(t, textDeltas, 1)
	assert.Equal(t, "Let me check", textDeltas[0].Delta)

	jsonDeltas := filterByType(evts, message.TypeInputJSONDelta)
	require.Len(t, jsonDeltas, 1)
	assert.Equal(t, `{"q":"test"}`, jsonDeltas[0].Delta)
}

// TestStream_ContextCancel는 ctx 취소 시 채널이 닫히는지 테스트한다.
func TestStream_ContextCancel(t *testing.T) {
	t.Parallel()
	// 무한 스트림을 simulate: 절대 끝나지 않는 읽기
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())

	out := make(chan message.StreamEvent, 32)
	go ParseAndConvert(ctx, io.NopCloser(pr), out, 60*time.Second)

	// cancel 후 채널이 닫히는지 확인
	cancel()
	pw.Close()

	// 채널이 닫혀야 함
	_, ok := <-out
	for ok {
		_, ok = <-out
	}
	// 여기까지 오면 채널이 닫힘 = pass
}

// TestStream_Empty는 빈 스트림([DONE]만 있는 경우)을 테스트한다.
func TestStream_Empty(t *testing.T) {
	t.Parallel()
	events := []string{`data: [DONE]`, ``}
	body := makeBody(events)
	out := make(chan message.StreamEvent, 32)
	ctx := context.Background()

	go ParseAndConvert(ctx, body, out, 60*time.Second)

	var evts []message.StreamEvent
	for e := range out {
		evts = append(evts, e)
	}
	stops := filterByType(evts, message.TypeMessageStop)
	require.Len(t, stops, 1)
}

// filterByType은 특정 타입의 이벤트만 필터링한다.
func filterByType(evts []message.StreamEvent, typ string) []message.StreamEvent {
	var result []message.StreamEvent
	for _, e := range evts {
		if e.Type == typ {
			result = append(result, e)
		}
	}
	return result
}
