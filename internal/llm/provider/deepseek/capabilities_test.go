package deepseek_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/deepseek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDeepSeekAdapter(t *testing.T) provider.Provider {
	t.Helper()
	creds := []*credential.PooledCredential{
		{ID: "c1", Provider: "deepseek", KeyringID: "k1", Status: credential.CredOK},
	}
	src := credential.NewDummySource(creds)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	require.NoError(t, err)

	store := provider.NewMemorySecretStore(map[string]string{"k1": "test-key"})

	adapter, err := deepseek.New(deepseek.Options{
		Pool:        pool,
		SecretStore: store,
		BaseURL:     "http://localhost:9999", // unreachable; only capabilities tested here
	})
	require.NoError(t, err)
	return adapter
}

// TestDeepSeek_AmendCapabilities verifies JSONMode=true, UserID=false for DeepSeek adapter.
// AC-AMEND-001 (deepseek row). REQ-AMEND-012.
func TestDeepSeek_AmendCapabilities(t *testing.T) {
	t.Parallel()

	adapter := makeDeepSeekAdapter(t)
	caps := adapter.Capabilities()

	assert.True(t, caps.JSONMode, "DeepSeek must report JSONMode=true (json_object supported)")
	assert.False(t, caps.UserID, "DeepSeek must report UserID=false (undocumented field)")

	// verify other parent capabilities are unchanged
	assert.True(t, caps.Streaming)
	assert.True(t, caps.Tools)
	assert.False(t, caps.Vision, "DeepSeek does not support Vision")
}
