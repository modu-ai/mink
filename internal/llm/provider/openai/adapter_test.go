package openai_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/provider/testhelper"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/llm/router"
	"github.com/modu-ai/goose/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeOpenAISSEBody는 OpenAI SSE 이벤트 목록으로 서버 응답 바디를 만든다.
func makeOpenAISSEBody(events []string) string {
	return strings.Join(events, "\n") + "\n"
}

// TestOpenAI_Stream_HappyPath는 AC-ADAPTER-004를 검증한다.
// OpenAI 어댑터가 SSE 스트림을 올바르게 파싱하여 StreamEvent를 반환하는지 테스트.
func TestOpenAI_Stream_HappyPath(t *testing.T) {
	t.Parallel()
	sseEvents := []string{
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":" from OpenAI"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 요청 헤더 검증
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer ")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeOpenAISSEBody(sseEvents))
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-test-key"})
	tracker := ratelimit.NewTracker()

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     srv.URL,
		Pool:        pool,
		Tracker:     tracker,
		SecretStore: secretStore,
		HTTPClient:  srv.Client(),
		Capabilities: provider.Capabilities{
			Streaming: true,
			Tools:     true,
		},
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	evts := testhelper.DrainStream(ctx, ch, 0)

	textDeltas := filterByType(evts, message.TypeTextDelta)
	require.Len(t, textDeltas, 2)
	assert.Equal(t, "Hello", textDeltas[0].Delta)
	assert.Equal(t, " from OpenAI", textDeltas[1].Delta)

	stops := filterByType(evts, message.TypeMessageStop)
	require.Len(t, stops, 1)
}

// TestOpenAI_Stream_ToolCall은 tool_call 스트리밍을 테스트한다.
func TestOpenAI_Stream_ToolCall(t *testing.T) {
	t.Parallel()
	sseEvents := []string{
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Seoul\"}"}}]},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-2","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
		``,
		`data: [DONE]`,
		``,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeOpenAISSEBody(sseEvents))
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-test-key"})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     srv.URL,
		Pool:        pool,
		SecretStore: secretStore,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "What's the weather?"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	evts := testhelper.DrainStream(ctx, ch, 0)

	blockStarts := filterByType(evts, message.TypeContentBlockStart)
	require.Len(t, blockStarts, 1)
	assert.Equal(t, "tool_use", blockStarts[0].BlockType)
	assert.Equal(t, "call_abc", blockStarts[0].ToolUseID)

	jsonDeltas := filterByType(evts, message.TypeInputJSONDelta)
	require.Len(t, jsonDeltas, 1)
}

// TestOpenAI_429Rotation은 429 응답 시 credential rotation 후 재시도를 테스트한다.
func TestOpenAI_429Rotation(t *testing.T) {
	t.Parallel()
	callCount := 0
	sseEvents := []string{
		`data: {"id":"chatcmpl-3","choices":[{"index":0,"delta":{"content":"OK"},"finish_reason":null}]}`,
		``,
		`data: [DONE]`,
		``,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeOpenAISSEBody(sseEvents))
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a", "cred-b"})
	secretStore := provider.NewMemorySecretStore(map[string]string{
		"kr-cred-a": "sk-key-a",
		"kr-cred-b": "sk-key-b",
	})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     srv.URL,
		Pool:        pool,
		SecretStore: secretStore,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	evts := testhelper.DrainStream(ctx, ch, 0)
	stops := filterByType(evts, message.TypeMessageStop)
	require.Len(t, stops, 1)
	assert.Equal(t, 2, callCount, "두 번째 시도에서 성공해야 함")
}

// TestOpenAI_Cancellation은 ctx 취소 시 스트림이 닫히는지 테스트한다.
func TestOpenAI_Cancellation(t *testing.T) {
	t.Parallel()
	// 스트림을 열어두지만 아무것도 보내지 않는 서버
	// 클라이언트 ctx 취소 시 채널이 닫혀야 함
	pr, pw := io.Pipe()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// flush headers
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// r.Context() 취소 시 pipe writer를 닫음
		select {
		case <-r.Context().Done():
			pw.Close()
		}
	}))
	defer srv.Close()
	defer pw.Close()
	defer pr.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-key"})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     srv.URL,
		Pool:        pool,
		SecretStore: secretStore,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	start := time.Now()
	ch, err := adapter.Stream(ctx, req)
	if err == nil {
		// ctx 취소 후 채널이 닫혀야 함
		testhelper.DrainStream(context.Background(), ch, 0)
	}
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 800*time.Millisecond, "취소 후 800ms 내에 완료되어야 함")
}

// TestOpenAI_NameAndCapabilities는 Name()과 Capabilities()를 검증한다.
func TestOpenAI_NameAndCapabilities(t *testing.T) {
	t.Parallel()
	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-key"})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		Pool:        pool,
		SecretStore: secretStore,
		Capabilities: provider.Capabilities{
			Streaming: true,
			Tools:     true,
			Vision:    true,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "openai", adapter.Name())
	assert.True(t, adapter.Capabilities().Streaming)
	assert.True(t, adapter.Capabilities().Vision)
}

// TestOpenAI_Complete는 Complete()가 스트림에서 텍스트를 수집하는지 검증한다.
func TestOpenAI_Complete(t *testing.T) {
	t.Parallel()
	sseEvents := []string{
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":"Complete"},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-1","choices":[{"index":0,"delta":{"content":" response"},"finish_reason":null}]}`,
		``,
		`data: [DONE]`,
		``,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, makeOpenAISSEBody(sseEvents))
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-key"})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     srv.URL,
		Pool:        pool,
		SecretStore: secretStore,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	resp, err := adapter.Complete(ctx, req)
	require.NoError(t, err)
	require.Len(t, resp.Message.Content, 1)
	assert.Equal(t, "Complete response", resp.Message.Content[0].Text)
}

// TestOpenAI_HeartbeatTimeout_EmitsError는 AC-013 heartbeat timeout을 검증한다.
// SSE 연결이 열려있지만 데이터가 전송되지 않을 때 200ms 내에 error 이벤트를 방출해야 한다.
func TestOpenAI_HeartbeatTimeout_EmitsError(t *testing.T) {
	t.Parallel()

	srv := testhelper.NewSilentSSEServer("")
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-test-key"})

	// HeartbeatTimeout: 200ms 주입
	adapter, err := openai.New(openai.OpenAIOptions{
		Name:             "openai",
		BaseURL:          srv.URL,
		Pool:             pool,
		SecretStore:      secretStore,
		HTTPClient:       srv.Client(),
		HeartbeatTimeout: 200 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "test"}}}},
	}

	start := time.Now()
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	events := testhelper.DrainStream(ctx, ch, 0)
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 2*time.Second, "heartbeat timeout 후 2초 내에 채널이 닫혀야 함")
	require.NotEmpty(t, events, "최소 1개 이벤트가 있어야 함")

	lastEvt := events[len(events)-1]
	assert.Equal(t, message.TypeError, lastEvt.Type, "마지막 이벤트가 error여야 함")
	assert.Contains(t, lastEvt.Error, "heartbeat", "에러 메시지에 'heartbeat'가 포함되어야 함")
}

// TestOpenAI_MissingPool는 Pool=nil 시 에러를 반환하는지 검증한다.
func TestOpenAI_MissingPool(t *testing.T) {
	t.Parallel()
	_, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		SecretStore: provider.NewMemorySecretStore(map[string]string{}),
	})
	assert.Error(t, err)
}

// TestOpenAI_MissingSecretStore는 SecretStore=nil 시 에러를 반환하는지 검증한다.
func TestOpenAI_MissingSecretStore(t *testing.T) {
	t.Parallel()
	pool := testhelper.FakePool(t, []string{"cred-a"})
	_, err := openai.New(openai.OpenAIOptions{
		Name: "openai",
		Pool: pool,
	})
	assert.Error(t, err)
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

// capturingHandler는 수신된 HTTP 요청을 캡처하는 핸들러이다.
type capturingHandler struct {
	capturedReq  *http.Request
	capturedBody []byte
	sseBody      string
}

func (h *capturingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.capturedReq = r
	body, _ := io.ReadAll(r.Body)
	h.capturedBody = body
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprint(w, h.sseBody)
}

// minimalSSEBody는 최소한의 유효한 SSE 응답 바디이다.
func minimalSSEBody() string {
	return makeOpenAISSEBody([]string{
		`data: {"id":"chatcmpl-x","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`,
		``,
		`data: [DONE]`,
		``,
	})
}

// TestOpenAI_ExtraHeaders_InjectedInRequest는 OpenAIOptions.ExtraHeaders가
// HTTP 요청 헤더에 주입되는지 검증한다 (SPEC-GOOSE-ADAPTER-002 prerequisite).
func TestOpenAI_ExtraHeaders_InjectedInRequest(t *testing.T) {
	t.Parallel()

	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-test-key"})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     srv.URL,
		Pool:        pool,
		SecretStore: secretStore,
		HTTPClient:  srv.Client(),
		ExtraHeaders: map[string]string{
			"HTTP-Referer": "https://example.com",
			"X-Title":      "test",
		},
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	require.NotNil(t, handler.capturedReq, "서버가 요청을 수신해야 함")
	assert.Equal(t, "https://example.com", handler.capturedReq.Header.Get("HTTP-Referer"),
		"HTTP-Referer 헤더가 주입되어야 함")
	assert.Equal(t, "test", handler.capturedReq.Header.Get("X-Title"),
		"X-Title 헤더가 주입되어야 함")
	// Authorization 헤더는 ExtraHeaders로 덮어쓰이지 않아야 함
	assert.Contains(t, handler.capturedReq.Header.Get("Authorization"), "Bearer ",
		"Authorization 헤더가 여전히 설정되어 있어야 함")
}

// TestOpenAI_ExtraRequestFields_MergedInBody는 CompletionRequest.ExtraRequestFields가
// request body JSON에 merge되는지 검증한다 (SPEC-GOOSE-ADAPTER-002 prerequisite).
func TestOpenAI_ExtraRequestFields_MergedInBody(t *testing.T) {
	t.Parallel()

	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-test-key"})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     srv.URL,
		Pool:        pool,
		SecretStore: secretStore,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}}},
		ExtraRequestFields: map[string]any{
			"thinking":     map[string]any{"type": "enabled"},
			"custom_param": float64(42),
		},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	require.NotNil(t, handler.capturedBody, "서버가 요청 바디를 수신해야 함")

	var bodyMap map[string]any
	require.NoError(t, json.Unmarshal(handler.capturedBody, &bodyMap))

	// ExtraRequestFields가 merge되어 있어야 함
	thinking, ok := bodyMap["thinking"].(map[string]any)
	require.True(t, ok, "thinking 필드가 map이어야 함")
	assert.Equal(t, "enabled", thinking["type"], "thinking.type == 'enabled'이어야 함")
	assert.Equal(t, float64(42), bodyMap["custom_param"], "custom_param == 42이어야 함")

	// 표준 필드도 존재해야 함
	assert.Equal(t, "gpt-4o", bodyMap["model"], "model 표준 필드가 존재해야 함")
	assert.NotNil(t, bodyMap["messages"], "messages 표준 필드가 존재해야 함")
	streamVal, _ := bodyMap["stream"].(bool)
	assert.True(t, streamVal, "stream 표준 필드가 true이어야 함")
}

// TestOpenAI_ExtraRequestFields_OverridesStandard는 ExtraRequestFields가 표준 필드를
// 덮어쓰는지 검증한다 (사용자 값 우선 정책).
// 설계 근거: 사용자가 ExtraRequestFields로 temperature를 명시한 경우 해당 값이 우선해야
// provider-specific 파라미터 조정 시 표준 필드 재설정 없이 단독 제어 가능하다.
func TestOpenAI_ExtraRequestFields_OverridesStandard(t *testing.T) {
	t.Parallel()

	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-test-key"})

	adapter, err := openai.New(openai.OpenAIOptions{
		Name:        "openai",
		BaseURL:     srv.URL,
		Pool:        pool,
		SecretStore: secretStore,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:       router.Route{Provider: "openai", Model: "gpt-4o"},
		Messages:    []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "hello"}}}},
		Temperature: 0.5,
		// ExtraRequestFields의 temperature가 표준 필드(0.5)를 덮어써야 함
		ExtraRequestFields: map[string]any{
			"temperature": 0.9,
		},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	require.NotNil(t, handler.capturedBody)

	var bodyMap map[string]any
	require.NoError(t, json.Unmarshal(handler.capturedBody, &bodyMap))

	// 사용자 ExtraRequestFields 값(0.9)이 표준 필드(0.5)를 덮어써야 함
	assert.Equal(t, 0.9, bodyMap["temperature"],
		"ExtraRequestFields.temperature(0.9)가 표준 Temperature(0.5)를 override해야 함")
}
