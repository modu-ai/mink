package deepseek_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/deepseek"
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

// TestDeepSeek_Capabilities는 DeepSeek 어댑터의 기능 목록을 검증한다.
// Vision=false, AdaptiveThinking=false가 핵심이다.
func TestDeepSeek_Capabilities(t *testing.T) {
	t.Parallel()
	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-ds-test"})

	adapter := deepseek.New(deepseek.Options{
		Pool:        pool,
		SecretStore: secretStore,
	})
	require.NotNil(t, adapter)

	assert.Equal(t, "deepseek", adapter.Name())

	caps := adapter.Capabilities()
	assert.True(t, caps.Streaming, "DeepSeek은 streaming 지원")
	assert.True(t, caps.Tools, "DeepSeek은 tool calling 지원")
	assert.False(t, caps.Vision, "DeepSeek은 vision 미지원")
	assert.False(t, caps.AdaptiveThinking, "DeepSeek은 adaptive thinking 미지원")
}

// TestDeepSeek_Stream_HappyPath는 DeepSeek 스트리밍 기본 동작을 검증한다.
func TestDeepSeek_Stream_HappyPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w,
			"data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n"+
				"data: [DONE]\n\n",
		)
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-ds-test"})

	adapter := deepseek.New(deepseek.Options{
		Pool:        pool,
		SecretStore: secretStore,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NotNil(t, adapter)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "deepseek", Model: "deepseek-chat"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hi"}}}},
	}

	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)

	evts := testhelper.DrainStream(ctx, ch, 0)
	var textDeltas []message.StreamEvent
	for _, e := range evts {
		if e.Type == message.TypeTextDelta {
			textDeltas = append(textDeltas, e)
		}
	}
	require.Len(t, textDeltas, 1)
	assert.Equal(t, "Hello", textDeltas[0].Delta)
}
