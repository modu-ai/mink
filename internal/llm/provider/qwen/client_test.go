package qwen_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/provider/qwen"
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

// TestQwen_DefaultRegionIntl는 AC-ADP2-010을 검증한다.
// Region 미지정 + 환경변수 없음 → dashscope-intl 사용.
func TestQwen_DefaultRegionIntl(t *testing.T) {
	t.Parallel()
	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-qwen-test"})

	adapter, err := qwen.New(qwen.Options{
		Pool:        pool,
		SecretStore: secretStore,
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)
	assert.Equal(t, "qwen", adapter.Name())
}

// TestQwen_RegionCN_UsesChineseBaseURL는 Region="cn"이면 CN 엔드포인트를 사용하는지 검증한다.
func TestQwen_RegionCN_UsesChineseBaseURL(t *testing.T) {
	t.Parallel()
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-qwen-test"})

	// BaseURL을 직접 override하면 region과 무관하게 테스트 서버로 보낸다
	adapter, err := qwen.New(qwen.Options{
		Pool:        pool,
		SecretStore: secretStore,
		Region:      qwen.RegionCN,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "qwen", Model: "qwen3-max"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	assert.Equal(t, "/chat/completions", requestPath)
}

// TestQwen_EnvVarFallback는 AC-ADP2-011을 검증한다.
// GOOSE_QWEN_REGION=cn 환경변수가 Region 기본값보다 우선됨.
func TestQwen_EnvVarFallback(t *testing.T) {
	t.Setenv("GOOSE_QWEN_REGION", "cn")

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-qwen-test"})

	adapter, err := qwen.New(qwen.Options{
		Pool:        pool,
		SecretStore: secretStore,
		// Region 미지정 — 환경변수가 우선되어야 함
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)
	assert.Equal(t, "qwen", adapter.Name())
}

// TestQwen_InvalidRegion_ReturnsError는 AC-ADP2-012를 검증한다.
// 잘못된 region → ErrInvalidRegion 반환 (REQ-ADP2-018).
func TestQwen_InvalidRegion_ReturnsError(t *testing.T) {
	t.Setenv("GOOSE_QWEN_REGION", "foo")

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-qwen-test"})

	_, err := qwen.New(qwen.Options{
		Pool:        pool,
		SecretStore: secretStore,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, qwen.ErrInvalidRegion)
}
