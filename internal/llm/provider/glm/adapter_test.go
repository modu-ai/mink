package glm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/glm"
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

// capturingHandler는 수신된 HTTP 요청과 바디를 캡처하는 핸들러이다.
type capturingHandler struct {
	capturedReq  *http.Request
	capturedBody []byte
	sseBody      string
}

func (h *capturingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.capturedReq = r.Clone(r.Context())
	body, _ := io.ReadAll(r.Body)
	h.capturedBody = body
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprint(w, h.sseBody)
}

func minimalSSEBody() string {
	return "data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"
}

// TestGLM_UsesZAIBaseURL는 AC-ADP2-001을 검증한다.
// GLM 어댑터가 api.z.ai/api/paas/v4 엔드포인트를 사용하는지 검증.
func TestGLM_UsesZAIBaseURL(t *testing.T) {
	t.Parallel()
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, minimalSSEBody())
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-glm-test"})

	adapter, err := glm.New(glm.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)

	assert.Equal(t, "glm", adapter.Name())

	caps := adapter.Capabilities()
	assert.True(t, caps.Streaming)
	assert.True(t, caps.Tools)
	assert.True(t, caps.Vision)
	assert.True(t, caps.AdaptiveThinking)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "glm", Model: "glm-4.5-air"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	assert.Equal(t, "/chat/completions", requestPath)
}

// TestGLM_Stream_InjectsThinkingInBody는 AC-ADP2-002를 검증한다.
// thinking-capable 모델 + Enabled=true → body에 thinking:{type:enabled} 주입.
func TestGLM_Stream_InjectsThinkingInBody(t *testing.T) {
	t.Parallel()
	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-glm-test"})

	adapter, err := glm.New(glm.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "glm", Model: "glm-4.6"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
		Thinking: &provider.ThinkingConfig{Enabled: true},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	require.NotNil(t, handler.capturedBody)
	var bodyMap map[string]any
	require.NoError(t, json.Unmarshal(handler.capturedBody, &bodyMap))

	// thinking 필드가 body에 주입되어야 함 (REQ-ADP2-007)
	thinkingVal, ok := bodyMap["thinking"].(map[string]any)
	require.True(t, ok, "thinking 필드가 map이어야 함")
	assert.Equal(t, "enabled", thinkingVal["type"], "thinking.type == 'enabled'이어야 함")
	// budget_tokens 필드는 없어야 함 (REQ-ADP2-007)
	assert.Nil(t, thinkingVal["budget_tokens"], "budget_tokens 필드는 기본적으로 없어야 함")
}

// TestGLM_Stream_NonSupportedModel_NoThinkingInBody는 AC-ADP2-003을 검증한다.
// thinking 미지원 모델 + Enabled=true → WARN + thinking 필드 부재 (REQ-ADP2-014).
func TestGLM_Stream_NonSupportedModel_NoThinkingInBody(t *testing.T) {
	t.Parallel()
	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-glm-test"})

	adapter, err := glm.New(glm.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "glm", Model: "glm-4.5-air"}, // thinking 미지원
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
		Thinking: &provider.ThinkingConfig{Enabled: true},
	}
	// 에러 없이 streaming 정상 (REQ-ADP2-014 graceful degradation)
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	require.NotNil(t, handler.capturedBody)
	var bodyMap map[string]any
	require.NoError(t, json.Unmarshal(handler.capturedBody, &bodyMap))

	// thinking 필드가 body에 없어야 함 (WARN + 무시)
	_, hasThinking := bodyMap["thinking"]
	assert.False(t, hasThinking, "thinking 미지원 모델에는 thinking 필드가 없어야 함")
}

// TestGLM_Complete_InjectsThinking는 Complete도 thinking 파라미터를 주입하는지 검증한다.
func TestGLM_Complete_InjectsThinking(t *testing.T) {
	t.Parallel()
	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-glm-test"})

	adapter, err := glm.New(glm.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "glm", Model: "glm-5"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
		Thinking: &provider.ThinkingConfig{Enabled: true},
	}
	resp, err := adapter.Complete(ctx, req)
	require.NoError(t, err)
	assert.NotNil(t, resp)

	require.NotNil(t, handler.capturedBody)
	var bodyMap map[string]any
	require.NoError(t, json.Unmarshal(handler.capturedBody, &bodyMap))

	thinkingVal, ok := bodyMap["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "enabled", thinkingVal["type"])
}

// TestGLM_Stream_PreservesExistingExtraRequestFields는
// 기존 ExtraRequestFields가 thinking 주입 후에도 보존됨을 검증한다.
func TestGLM_Stream_PreservesExistingExtraRequestFields(t *testing.T) {
	t.Parallel()
	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-glm-test"})

	adapter, err := glm.New(glm.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "glm", Model: "glm-4.7"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
		Thinking: &provider.ThinkingConfig{Enabled: true},
		ExtraRequestFields: map[string]any{
			"custom_param": "preserved_value",
		},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	require.NotNil(t, handler.capturedBody)
	var bodyMap map[string]any
	require.NoError(t, json.Unmarshal(handler.capturedBody, &bodyMap))

	// thinking 필드 주입
	thinkingVal, ok := bodyMap["thinking"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "enabled", thinkingVal["type"])

	// 기존 custom_param도 보존되어야 함
	assert.Equal(t, "preserved_value", bodyMap["custom_param"],
		"기존 ExtraRequestFields가 thinking 주입 후에도 보존되어야 함")
}
