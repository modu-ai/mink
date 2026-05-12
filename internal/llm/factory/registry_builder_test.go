package factory_test

import (
	"testing"

	"github.com/modu-ai/mink/internal/llm/credential"
	"github.com/modu-ai/mink/internal/llm/factory"
	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegisterAllProviders_NoError는 AC-ADP2-016/017을 검증한다.
// 유효한 입력 시 15개(SPEC-001 6종 + SPEC-002 9종) provider가 에러 없이 등록됨.
func TestRegisterAllProviders_NoError(t *testing.T) {
	t.Parallel()
	reg := provider.NewRegistry()
	pool := newFakePool(t)
	tracker := ratelimit.NewTracker()
	secretStore := provider.NewMemorySecretStore(map[string]string{})

	err := factory.RegisterAllProviders(reg, pool, tracker, secretStore, nil)
	require.NoError(t, err)

	names := reg.Names()
	// 15개 provider (SPEC-001 6종: anthropic/openai/google/xai/deepseek/ollama + SPEC-002 9종)
	assert.Len(t, names, 15, "RegisterAllProviders: 15개 provider 등록 기대")

	// SPEC-001 6종 모두 등록 확인
	spec001Providers := []string{
		"anthropic", "google", "openai", "xai", "deepseek", "ollama",
	}
	for _, name := range spec001Providers {
		_, ok := reg.Get(name)
		assert.True(t, ok, "SPEC-001 provider %q가 등록되어야 함", name)
	}

	// SPEC-002 9종 모두 등록 확인
	spec002Providers := []string{
		"glm", "groq", "openrouter", "together", "fireworks", "cerebras", "mistral", "qwen", "kimi",
	}
	for _, name := range spec002Providers {
		_, ok := reg.Get(name)
		assert.True(t, ok, "SPEC-002 provider %q가 등록되어야 함", name)
	}
}

// TestRegisterAllProviders_IncludesAnthropicAndGoogle는 AC-ADP2-016/017을 직접 검증한다.
// anthropic과 google이 RegisterAllProviders에 포함되는지 확인한다.
func TestRegisterAllProviders_IncludesAnthropicAndGoogle(t *testing.T) {
	t.Parallel()

	t.Run("anthropic 등록", func(t *testing.T) {
		t.Parallel()
		reg := provider.NewRegistry()
		pool := newFakePool(t)
		tracker := ratelimit.NewTracker()
		secretStore := provider.NewMemorySecretStore(map[string]string{})

		require.NoError(t, factory.RegisterAllProviders(reg, pool, tracker, secretStore, nil))
		_, ok := reg.Get("anthropic")
		assert.True(t, ok, "anthropic provider가 등록되어야 함 (REQ-ADP2-005)")
	})

	t.Run("google 등록", func(t *testing.T) {
		t.Parallel()
		reg := provider.NewRegistry()
		pool := newFakePool(t)
		tracker := ratelimit.NewTracker()
		secretStore := provider.NewMemorySecretStore(map[string]string{})

		require.NoError(t, factory.RegisterAllProviders(reg, pool, tracker, secretStore, nil))
		_, ok := reg.Get("google")
		assert.True(t, ok, "google provider가 등록되어야 함 (REQ-ADP2-005)")
	})

	t.Run("총 15개 등록", func(t *testing.T) {
		t.Parallel()
		reg := provider.NewRegistry()
		pool := newFakePool(t)
		tracker := ratelimit.NewTracker()
		secretStore := provider.NewMemorySecretStore(map[string]string{})

		require.NoError(t, factory.RegisterAllProviders(reg, pool, tracker, secretStore, nil))
		assert.Len(t, reg.Names(), 15, "SPEC-001 6종 + SPEC-002 9종 = 15개")
	})
}

// TestRegisterAllProviders_UniqueNames는 등록된 provider 이름이 모두 고유한지 검증한다.
func TestRegisterAllProviders_UniqueNames(t *testing.T) {
	t.Parallel()
	reg := provider.NewRegistry()
	pool := newFakePool(t)
	tracker := ratelimit.NewTracker()
	secretStore := provider.NewMemorySecretStore(map[string]string{})

	err := factory.RegisterAllProviders(reg, pool, tracker, secretStore, nil)
	require.NoError(t, err)

	names := reg.Names()
	nameSet := make(map[string]bool, len(names))
	for _, name := range names {
		assert.False(t, nameSet[name], "provider 이름 중복: %q", name)
		nameSet[name] = true
	}
}

// TestRegisterAllProviders_DuplicateProvider는 AC-ADP2-018을 검증한다.
// 이미 등록된 provider를 다시 RegisterAllProviders로 시도하면 에러 반환.
func TestRegisterAllProviders_DuplicateProvider(t *testing.T) {
	t.Parallel()
	reg := provider.NewRegistry()
	pool := newFakePool(t)
	tracker := ratelimit.NewTracker()
	secretStore := provider.NewMemorySecretStore(map[string]string{})

	// 1회 등록
	err := factory.RegisterAllProviders(reg, pool, tracker, secretStore, nil)
	require.NoError(t, err)

	// 2회 시도 — 이름 충돌로 에러 기대 (REQ-ADP2-016)
	err = factory.RegisterAllProviders(reg, pool, tracker, secretStore, nil)
	require.Error(t, err, "중복 등록 시 에러가 반환되어야 함")
}

// newFakePool은 테스트용 빈 credential pool을 생성한다.
func newFakePool(t *testing.T) *credential.CredentialPool {
	t.Helper()
	src := credential.NewDummySource(nil)
	pool, err := credential.New(src, credential.NewRoundRobinStrategy())
	require.NoError(t, err)
	return pool
}
