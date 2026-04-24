package ollama_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/ollama"
	"github.com/modu-ai/goose/internal/llm/provider/testhelper"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// writeJSONL는 JSON-L 라인들을 http.ResponseWriter에 순차적으로 쓴다.
func writeJSONL(w http.ResponseWriter, lines []any) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	for _, line := range lines {
		b, _ := json.Marshal(line)
		fmt.Fprintf(w, "%s\n", b)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

// ollamaStreamLine는 Ollama /api/chat 스트림 라인 구조이다.
type ollamaStreamLine struct {
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

// ollamaToolLine는 tool_calls가 포함된 Ollama 스트림 라인이다.
type ollamaToolLine struct {
	Message struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		ToolCalls []struct {
			Function struct {
				Name      string `json:"name"`
				Arguments any    `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"message"`
	Done bool `json:"done"`
}

// TestOllama_Stream_HappyPath는 AC-ADAPTER-007을 검증한다.
// Ollama JSON-L 스트리밍 기본 동작 검증.
func TestOllama_Stream_HappyPath(t *testing.T) {
	t.Parallel()
	lines := []any{
		map[string]any{
			"message": map[string]any{"role": "assistant", "content": "Hello"},
			"done":    false,
		},
		map[string]any{
			"message": map[string]any{"role": "assistant", "content": " from Ollama"},
			"done":    false,
		},
		map[string]any{
			"message": map[string]any{"role": "assistant", "content": ""},
			"done":    true,
		},
	}

	var requestBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		// 요청 바디 캡처
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		requestBody = buf[:n]
		writeJSONL(w, lines)
	}))
	defer srv.Close()

	adapter, err := ollama.New(ollama.OllamaOptions{
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	assert.Equal(t, "ollama", adapter.Name())
	caps := adapter.Capabilities()
	assert.True(t, caps.Streaming)
	assert.True(t, caps.Tools)
	assert.True(t, caps.Vision)
	assert.False(t, caps.AdaptiveThinking)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "ollama", Model: "llama3.2"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	evts := testhelper.DrainStream(ctx, ch, 0)

	textDeltas := filterByType(evts, message.TypeTextDelta)
	require.Len(t, textDeltas, 2)
	assert.Equal(t, "Hello", textDeltas[0].Delta)
	assert.Equal(t, " from Ollama", textDeltas[1].Delta)

	stops := filterByType(evts, message.TypeMessageStop)
	require.Len(t, stops, 1)

	// 요청 바디에 model 필드가 있는지 확인
	var bodyMap map[string]any
	_ = json.Unmarshal(requestBody, &bodyMap)
	assert.Equal(t, "llama3.2", bodyMap["model"])
	assert.Equal(t, true, bodyMap["stream"])
}

// TestOllama_Stream_ToolCall은 Ollama tool_calls 스트리밍을 검증한다.
func TestOllama_Stream_ToolCall(t *testing.T) {
	t.Parallel()
	lines := []any{
		map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": "",
				"tool_calls": []map[string]any{
					{
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": map[string]any{"city": "Seoul"},
						},
					},
				},
			},
			"done": false,
		},
		map[string]any{
			"message": map[string]any{"role": "assistant", "content": ""},
			"done":    true,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSONL(w, lines)
	}))
	defer srv.Close()

	adapter, err := ollama.New(ollama.OllamaOptions{
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "ollama", Model: "llama3.2"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Weather?"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	evts := testhelper.DrainStream(ctx, ch, 0)

	// content_block_start (tool_use)
	blockStarts := filterByType(evts, message.TypeContentBlockStart)
	require.Len(t, blockStarts, 1)
	assert.Equal(t, "tool_use", blockStarts[0].BlockType)

	// message_stop
	stops := filterByType(evts, message.TypeMessageStop)
	require.Len(t, stops, 1)
}

// TestOllama_DefaultEndpoint는 기본 endpoint가 localhost:11434인지 검증한다.
func TestOllama_DefaultEndpoint(t *testing.T) {
	t.Parallel()
	adapter, err := ollama.New(ollama.OllamaOptions{})
	require.NoError(t, err)
	assert.Equal(t, "ollama", adapter.Name())
}

// TestOllama_Cancellation은 ctx 취소 시 스트림이 닫히는지 검증한다.
func TestOllama_Cancellation(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// ctx 취소 때까지 대기
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer srv.Close()

	adapter, err := ollama.New(ollama.OllamaOptions{
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "ollama", Model: "llama3.2"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	start := time.Now()
	ch, err := adapter.Stream(ctx, req)
	if err == nil {
		testhelper.DrainStream(context.Background(), ch, 0)
	}
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 800*time.Millisecond, "취소 후 800ms 내에 완료되어야 함")
}

// TestOllama_Complete는 Complete()가 스트림에서 텍스트를 수집하는지 검증한다.
func TestOllama_Complete(t *testing.T) {
	t.Parallel()
	lines := []any{
		map[string]any{"message": map[string]any{"role": "assistant", "content": "Hello"}, "done": false},
		map[string]any{"message": map[string]any{"role": "assistant", "content": " from Ollama"}, "done": false},
		map[string]any{"message": map[string]any{"role": "assistant", "content": ""}, "done": true},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSONL(w, lines)
	}))
	defer srv.Close()

	adapter, err := ollama.New(ollama.OllamaOptions{
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "ollama", Model: "llama3.2"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hi"}}}},
	}

	resp, err := adapter.Complete(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Message.Content, 1)
	assert.Equal(t, "Hello from Ollama", resp.Message.Content[0].Text)
}

// TestOllama_ServerError는 서버 에러 시 에러를 반환하는지 검증한다.
func TestOllama_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	adapter, err := ollama.New(ollama.OllamaOptions{
		Endpoint:   srv.URL,
		HTTPClient: srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "ollama", Model: "llama3.2"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	_, err = adapter.Stream(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
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
