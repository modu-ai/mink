package openrouter_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/openrouter"
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

// capturingHandler는 수신된 HTTP 요청을 캡처하는 핸들러이다.
type capturingHandler struct {
	capturedReq *http.Request
	sseBody     string
}

func (h *capturingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.capturedReq = r.Clone(r.Context())
	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprint(w, h.sseBody)
}

func minimalSSEBody() string {
	return "data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"
}

// TestOpenRouter_UsesCustomBaseURL는 OpenRouter 어댑터가 올바른 BaseURL을 사용하는지 검증한다.
func TestOpenRouter_UsesCustomBaseURL(t *testing.T) {
	t.Parallel()
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, minimalSSEBody())
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-openrouter-test"})

	adapter, err := openrouter.New(openrouter.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)

	assert.Equal(t, "openrouter", adapter.Name())

	caps := adapter.Capabilities()
	assert.True(t, caps.Streaming)
	assert.True(t, caps.Tools)
	assert.True(t, caps.Vision) // OpenRouter는 300+ 모델 gateway — vision 지원

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openrouter", Model: "deepseek/deepseek-r1:free"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	assert.Equal(t, "/chat/completions", requestPath)
}

// TestOpenRouter_InjectsRankingHeaders는 AC-ADP2-005를 검증한다.
// HTTPReferer와 XTitle 옵션이 HTTP 요청 헤더에 주입되는지 확인.
func TestOpenRouter_InjectsRankingHeaders(t *testing.T) {
	t.Parallel()
	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-openrouter-test"})

	adapter, err := openrouter.New(openrouter.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
		HTTPReferer: "https://goose.modu-ai.dev",
		XTitle:      "GOOSE CLI",
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openrouter", Model: "deepseek/deepseek-r1:free"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	require.NotNil(t, handler.capturedReq)
	assert.Equal(t, "https://goose.modu-ai.dev", handler.capturedReq.Header.Get("HTTP-Referer"),
		"HTTP-Referer 헤더가 주입되어야 함 (REQ-ADP2-008)")
	assert.Equal(t, "GOOSE CLI", handler.capturedReq.Header.Get("X-Title"),
		"X-Title 헤더가 주입되어야 함 (REQ-ADP2-008)")
	// Authorization 헤더가 유지되어야 함
	assert.Contains(t, handler.capturedReq.Header.Get("Authorization"), "Bearer ")
}

// TestOpenRouter_OmitsHeadersWhenEmpty는 HTTPReferer/XTitle이 빈 값이면 헤더가 주입되지 않음을 검증한다.
func TestOpenRouter_OmitsHeadersWhenEmpty(t *testing.T) {
	t.Parallel()
	handler := &capturingHandler{sseBody: minimalSSEBody()}
	srv := httptest.NewServer(handler)
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-openrouter-test"})

	// HTTPReferer, XTitle 미설정
	adapter, err := openrouter.New(openrouter.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "openrouter", Model: "openai/gpt-4o"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	require.NotNil(t, handler.capturedReq)
	// 빈 옵션이면 OpenRouter ranking 헤더가 주입되지 않아야 함
	assert.Empty(t, handler.capturedReq.Header.Get("HTTP-Referer"),
		"HTTPReferer 미설정 시 헤더가 없어야 함")
	assert.Empty(t, handler.capturedReq.Header.Get("X-Title"),
		"XTitle 미설정 시 헤더가 없어야 함")
}
