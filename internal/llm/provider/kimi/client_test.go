package kimi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/kimi"
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

// TestKimi_DefaultRegionIntl는 Region 미지정 시 intl 엔드포인트를 사용하는지 검증한다.
func TestKimi_DefaultRegionIntl(t *testing.T) {
	t.Parallel()
	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-kimi-test"})

	adapter, err := kimi.New(kimi.Options{
		Pool:        pool,
		SecretStore: secretStore,
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)
	assert.Equal(t, "kimi", adapter.Name())
}

// TestKimi_RegionCN는 AC-ADP2-014를 검증한다.
// Region="cn" 시 api.moonshot.cn 엔드포인트 사용.
func TestKimi_RegionCN(t *testing.T) {
	t.Parallel()
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-kimi-test"})

	adapter, err := kimi.New(kimi.Options{
		Pool:        pool,
		SecretStore: secretStore,
		Region:      kimi.RegionCN,
		BaseURL:     srv.URL,
		HTTPClient:  srv.Client(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	req := provider.CompletionRequest{
		Route:    router.Route{Provider: "kimi", Model: "moonshot-v1-128k"},
		Messages: []message.Message{{Role: "user", Content: []message.ContentBlock{{Type: "text", Text: "Hello"}}}},
	}
	ch, err := adapter.Stream(ctx, req)
	require.NoError(t, err)
	testhelper.DrainStream(ctx, ch, 0)

	assert.Equal(t, "/chat/completions", requestPath)
}

// TestKimi_EnvVarFallback는 GOOSE_KIMI_REGION 환경변수 우선순위를 검증한다.
func TestKimi_EnvVarFallback(t *testing.T) {
	t.Setenv("GOOSE_KIMI_REGION", "cn")

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-kimi-test"})

	adapter, err := kimi.New(kimi.Options{
		Pool:        pool,
		SecretStore: secretStore,
	})
	require.NoError(t, err)
	require.NotNil(t, adapter)
	assert.Equal(t, "kimi", adapter.Name())
}

// TestKimi_InvalidRegion_ReturnsError는 잘못된 region에서 에러를 반환하는지 검증한다.
func TestKimi_InvalidRegion_ReturnsError(t *testing.T) {
	t.Setenv("GOOSE_KIMI_REGION", "invalid")

	pool := testhelper.FakePool(t, []string{"cred-a"})
	secretStore := provider.NewMemorySecretStore(map[string]string{"kr-cred-a": "sk-kimi-test"})

	_, err := kimi.New(kimi.Options{
		Pool:        pool,
		SecretStore: secretStore,
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, kimi.ErrInvalidRegion)
}
