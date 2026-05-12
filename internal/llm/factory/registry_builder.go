// Package factory의 registry_builder.go: 15 provider 일괄 등록 헬퍼.
// import cycle 방지를 위해 provider 서브패키지와 분리된 factory 패키지에 위치한다.
// SPEC-001 6종(anthropic/openai/google/xai/deepseek/ollama) + SPEC-002 9종 provider를 ProviderRegistry에 등록한다.
// SPEC-GOOSE-ADAPTER-002 M5
package factory

import (
	"fmt"

	"github.com/modu-ai/mink/internal/llm/provider"
	anthropicprovider "github.com/modu-ai/mink/internal/llm/provider/anthropic"
	"github.com/modu-ai/mink/internal/llm/provider/cerebras"
	"github.com/modu-ai/mink/internal/llm/provider/deepseek"
	"github.com/modu-ai/mink/internal/llm/provider/fireworks"
	glmprovider "github.com/modu-ai/mink/internal/llm/provider/glm"
	googleprovider "github.com/modu-ai/mink/internal/llm/provider/google"
	"github.com/modu-ai/mink/internal/llm/provider/groq"
	"github.com/modu-ai/mink/internal/llm/provider/kimi"
	"github.com/modu-ai/mink/internal/llm/provider/mistral"
	"github.com/modu-ai/mink/internal/llm/provider/ollama"
	"github.com/modu-ai/mink/internal/llm/provider/openai"
	"github.com/modu-ai/mink/internal/llm/provider/openrouter"
	"github.com/modu-ai/mink/internal/llm/provider/qwen"
	"github.com/modu-ai/mink/internal/llm/provider/together"
	"github.com/modu-ai/mink/internal/llm/provider/xai"
	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"go.uber.org/zap"

	"github.com/modu-ai/mink/internal/llm/credential"
)

// RegisterAllProviders는 15 provider 인스턴스를 생성하여 reg에 등록한다.
// SPEC-001 6종(anthropic/openai/google/xai/deepseek/ollama)
// + SPEC-002 9종(glm/groq/openrouter/together/fireworks/cerebras/mistral/qwen/kimi).
//
// Ollama는 credential 없이 동작한다. 나머지 provider는 pool + secretStore가 필요하다.
// 첫 번째 에러에서 즉시 반환한다 (REQ-ADP2-010).
// 이름 중복 시 에러 반환 (REQ-ADP2-016).
//
// @MX:ANCHOR: [AUTO] 15 provider 일괄 등록 진입점
// @MX:REASON: 통합 테스트, factory.NewDefaultRegistry() 확장 시 fan_in >= 3 예상
func RegisterAllProviders(
	reg *provider.ProviderRegistry,
	pool *credential.CredentialPool,
	tracker *ratelimit.Tracker,
	secretStore provider.SecretStore,
	logger *zap.Logger,
) error {
	type factoryFn func() (provider.Provider, error)

	factories := []factoryFn{
		// SPEC-001 providers — anthropic (REQ-ADP2-005, AC-ADP2-016)
		func() (provider.Provider, error) {
			return anthropicprovider.New(anthropicprovider.AnthropicOptions{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		// SPEC-001 providers — google (REQ-ADP2-005, AC-ADP2-017)
		func() (provider.Provider, error) {
			return googleprovider.New(googleprovider.GoogleOptions{
				Pool:        pool,
				SecretStore: secretStore,
				Tracker:     tracker,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return openai.New(openai.OpenAIOptions{
				Name:        "openai",
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Capabilities: provider.Capabilities{
					Streaming: true, Tools: true, Vision: true,
				},
				Logger: logger,
			})
		},
		func() (provider.Provider, error) {
			return xai.New(xai.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return deepseek.New(deepseek.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return ollama.New(ollama.OllamaOptions{Logger: logger})
		},
		// SPEC-002 providers
		func() (provider.Provider, error) {
			return glmprovider.New(glmprovider.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return groq.New(groq.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return openrouter.New(openrouter.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return together.New(together.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return fireworks.New(fireworks.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return cerebras.New(cerebras.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return mistral.New(mistral.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return qwen.New(qwen.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
		func() (provider.Provider, error) {
			return kimi.New(kimi.Options{
				Pool:        pool,
				Tracker:     tracker,
				SecretStore: secretStore,
				Logger:      logger,
			})
		},
	}

	for _, factory := range factories {
		p, err := factory()
		if err != nil {
			return fmt.Errorf("RegisterAllProviders: provider 생성 실패: %w", err)
		}
		if err := reg.Register(p); err != nil {
			return fmt.Errorf("RegisterAllProviders: provider %q 등록 실패: %w", p.Name(), err)
		}
	}

	return nil
}
