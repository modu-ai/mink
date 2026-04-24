package google_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/google"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	// genai SDK가 사용하는 go.opencensus.io 백그라운드 goroutine을 필터링한다.
	// 이 goroutine은 패키지 init 시 시작되며 우리 코드와 무관하다.
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
	)
}

// fakeGeminiClient는 테스트용 gemini 클라이언트 구현이다.
type fakeGeminiClient struct {
	chunks []google.FakeChunk
	err    error
}

func (f *fakeGeminiClient) GenerateStream(ctx context.Context, req google.GeminiRequest) (google.GeminiStream, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &fakeGeminiStream{chunks: f.chunks, ctx: ctx}, nil
}

// fakeGeminiStream는 테스트용 gemini 스트림 구현이다.
type fakeGeminiStream struct {
	chunks []google.FakeChunk
	idx    int
	ctx    context.Context
}

func (s *fakeGeminiStream) Next() (*google.GeminiChunk, error) {
	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	default:
	}
	if s.idx >= len(s.chunks) {
		return nil, google.ErrStreamDone
	}
	chunk := s.chunks[s.idx]
	s.idx++
	return &google.GeminiChunk{
		Text:     chunk.Text,
		IsDone:   chunk.IsDone,
		HasTool:  chunk.HasTool,
		ToolName: chunk.ToolName,
		ToolArgs: chunk.ToolArgs,
	}, nil
}

func (s *fakeGeminiStream) Close() {}

// TestGoogle_GeminiStream_HappyPath는 AC-ADAPTER-006을 검증한다.
// Google Gemini 스트리밍 기본 동작 검증.
func TestGoogle_GeminiStream_HappyPath(t *testing.T) {
	t.Parallel()
	fakeClient := &fakeGeminiClient{
		chunks: []google.FakeChunk{
			{Text: "Hello"},
			{Text: " from Gemini"},
			{IsDone: true},
		},
	}

	adapter, err := google.New(google.GoogleOptions{
		ClientFactory: func(_ string) google.GeminiClientIface {
			return fakeClient
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "google", adapter.Name())
	caps := adapter.Capabilities()
	assert.True(t, caps.Streaming)
	assert.True(t, caps.Tools)
	assert.True(t, caps.Vision)
	assert.False(t, caps.AdaptiveThinking)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "google", Model: "gemini-2.0-flash"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	var evts []message.StreamEvent
	for e := range ch {
		evts = append(evts, e)
	}

	var textDeltas []message.StreamEvent
	for _, e := range evts {
		if e.Type == message.TypeTextDelta {
			textDeltas = append(textDeltas, e)
		}
	}
	require.Len(t, textDeltas, 2)
	assert.Equal(t, "Hello", textDeltas[0].Delta)
	assert.Equal(t, " from Gemini", textDeltas[1].Delta)

	stops := filterByType(evts, message.TypeMessageStop)
	require.Len(t, stops, 1)
}

// TestGoogle_GeminiStream_ToolCall은 Google Gemini tool_call 스트리밍을 검증한다.
func TestGoogle_GeminiStream_ToolCall(t *testing.T) {
	t.Parallel()
	fakeClient := &fakeGeminiClient{
		chunks: []google.FakeChunk{
			{HasTool: true, ToolName: "get_weather", ToolArgs: `{"city":"Seoul"}`},
			{IsDone: true},
		},
	}

	adapter, err := google.New(google.GoogleOptions{
		ClientFactory: func(_ string) google.GeminiClientIface {
			return fakeClient
		},
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "google", Model: "gemini-2.0-flash"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Weather?"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	var evts []message.StreamEvent
	for e := range ch {
		evts = append(evts, e)
	}

	// content_block_start (tool_use)
	blockStarts := filterByType(evts, message.TypeContentBlockStart)
	require.Len(t, blockStarts, 1)
	assert.Equal(t, "tool_use", blockStarts[0].BlockType)

	// input_json_delta
	jsonDeltas := filterByType(evts, message.TypeInputJSONDelta)
	require.Len(t, jsonDeltas, 1)
	assert.Equal(t, `{"city":"Seoul"}`, jsonDeltas[0].Delta)
}

// TestGoogle_Cancellation은 ctx 취소 시 스트림이 닫히는지 검증한다.
func TestGoogle_Cancellation(t *testing.T) {
	t.Parallel()
	// Next()가 ctx를 확인하는 스트림
	ctx, cancel := context.WithCancel(context.Background())

	fakeClient := &fakeGeminiClient{
		chunks: []google.FakeChunk{
			{Text: "first"},
			// 이후는 ctx 취소 후 ErrStreamDone 반환
		},
	}

	adapter, err := google.New(google.GoogleOptions{
		ClientFactory: func(_ string) google.GeminiClientIface {
			return fakeClient
		},
	})
	require.NoError(t, err)

	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "google", Model: "gemini-2.0-flash"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	// 첫 이벤트 수신 후 취소
	e, ok := <-ch
	assert.True(t, ok)
	assert.Equal(t, message.TypeTextDelta, e.Type)
	cancel()

	// 채널이 닫힐 때까지 drain
	for range ch {
	}
	// 여기까지 오면 채널이 닫힘 = pass
}

// TestGoogle_GenerateError는 Generate 에러 시 에러 스트림을 반환하는지 검증한다.
func TestGoogle_GenerateError(t *testing.T) {
	t.Parallel()
	fakeClient := &fakeGeminiClient{
		err: errors.New("API error: quota exceeded"),
	}

	adapter, err := google.New(google.GoogleOptions{
		ClientFactory: func(_ string) google.GeminiClientIface {
			return fakeClient
		},
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "google", Model: "gemini-2.0-flash"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	_, err = adapter.Stream(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quota exceeded")
}

// TestGoogle_Complete는 Complete()가 스트림에서 텍스트를 수집하는지 검증한다.
func TestGoogle_Complete(t *testing.T) {
	t.Parallel()
	fakeClient := &fakeGeminiClient{
		chunks: []google.FakeChunk{
			{Text: "Complete"},
			{Text: " response"},
			{IsDone: true},
		},
	}

	adapter, err := google.New(google.GoogleOptions{
		ClientFactory: func(_ string) google.GeminiClientIface {
			return fakeClient
		},
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "google", Model: "gemini-2.0-flash"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	resp, err := adapter.Complete(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Message.Content, 1)
	assert.Equal(t, "Complete response", resp.Message.Content[0].Text)
}

func filterByType(evts []message.StreamEvent, typ string) []message.StreamEvent {
	var result []message.StreamEvent
	for _, e := range evts {
		if e.Type == typ {
			result = append(result, e)
		}
	}
	return result
}
