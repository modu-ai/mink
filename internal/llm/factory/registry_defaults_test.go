package factory_test

import (
	"testing"

	"github.com/modu-ai/goose/internal/llm/factory"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestNewDefaultRegistry_AllProviders는 모든 provider가 인스턴스화되는지 검증한다.
func TestNewDefaultRegistry_AllProviders(t *testing.T) {
	t.Parallel()
	opts := factory.DefaultRegistryOptions{
		SecretStore:      provider.NewMemorySecretStore(map[string]string{}),
		EnabledProviders: []string{"openai", "xai", "deepseek", "ollama"},
	}

	reg, err := factory.NewDefaultRegistry(opts)
	require.NoError(t, err)
	require.NotNil(t, reg)

	names := reg.Names()
	assert.Contains(t, names, "openai", "openai이 등록되어야 함")
	assert.Contains(t, names, "xai", "xai가 등록되어야 함")
	assert.Contains(t, names, "deepseek", "deepseek이 등록되어야 함")
	assert.Contains(t, names, "ollama", "ollama가 등록되어야 함")
}

// TestNewDefaultRegistry_VisionCapabilities는 provider별 vision capability 설정을 검증한다.
func TestNewDefaultRegistry_VisionCapabilities(t *testing.T) {
	t.Parallel()
	opts := factory.DefaultRegistryOptions{
		SecretStore:      provider.NewMemorySecretStore(map[string]string{}),
		EnabledProviders: []string{"openai", "deepseek"},
	}

	reg, err := factory.NewDefaultRegistry(opts)
	require.NoError(t, err)

	openaiP, ok := reg.Get("openai")
	require.True(t, ok)
	assert.True(t, openaiP.Capabilities().Vision, "OpenAI는 vision 지원")

	deepseekP, ok := reg.Get("deepseek")
	require.True(t, ok)
	assert.False(t, deepseekP.Capabilities().Vision, "DeepSeek은 vision 미지원")
}

// TestNewDefaultRegistry_OllamaNoCredential는 Ollama가 credential 없이 동작하는지 검증한다.
func TestNewDefaultRegistry_OllamaNoCredential(t *testing.T) {
	t.Parallel()
	opts := factory.DefaultRegistryOptions{
		SecretStore:      nil, // SecretStore 없어도 ollama는 동작해야 함
		EnabledProviders: []string{"ollama"},
	}

	reg, err := factory.NewDefaultRegistry(opts)
	require.NoError(t, err)

	ollamaP, ok := reg.Get("ollama")
	require.True(t, ok)
	assert.Equal(t, "ollama", ollamaP.Name())
}

// TestNewDefaultRegistry_EmptyEnabled는 빈 EnabledProviders 시 에러 없이 빈 레지스트리를 반환하는지 검증한다.
func TestNewDefaultRegistry_EmptyEnabled(t *testing.T) {
	t.Parallel()
	opts := factory.DefaultRegistryOptions{
		SecretStore:      provider.NewMemorySecretStore(map[string]string{}),
		EnabledProviders: []string{},
	}

	reg, err := factory.NewDefaultRegistry(opts)
	require.NoError(t, err)
	assert.Empty(t, reg.Names())
}
