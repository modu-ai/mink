package cerebras_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/provider/cerebras"
	"github.com/modu-ai/mink/internal/llm/provider/testhelper"
	"github.com/modu-ai/mink/internal/llm/router"
	"github.com/modu-ai/mink/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestCerebras_UsesCustomBaseURL는 AC-ADP2-008을 검증한다.
// Cerebras 어댑터가 https://api.cerebras.ai/v1를 기본으로 사용하고,
// 테스트에서 override 시 올바른 경로에 요청하는지 검증.
func TestCerebras_UsesCustomBaseURL(t *testing.T) {
	t.Parallel()
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-cerebras-test"})

	adapter, err := cerebras.New(cerebras.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)

	// Name 검증
	assert.Equal(t, "cerebras", adapter.Name())

	// Capabilities 검증
	caps := adapter.Capabilities()
	assert.True(t, caps.Streaming)
	assert.True(t, caps.Tools)
	assert.False(t, caps.Vision) // Cerebras은 vision 미지원
	assert.False(t, caps.AdaptiveThinking)

	// 실제 요청 경로 검증
	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "cerebras", Model: "llama-3.3-70b"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	assert.Equal(t, "/chat/completions", requestPath, "요청이 /chat/completions 경로로 전송되어야 함")
}

// TestCerebras_DefaultName은 Cerebras 어댑터의 기본 이름을 검증한다.
func TestCerebras_DefaultName(t *testing.T) {
	t.Parallel()
	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-cerebras-test"})

	adapter, err := cerebras.New(cerebras.Options{
		Pool:        pool,
		SecretStore: secretStore,
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)
	assert.Equal(t, "cerebras", adapter.Name())
}
