// Package factory는 provider registry 초기화 팩토리를 제공한다.
// import cycle을 피하기 위해 provider 패키지와 분리된 별도 패키지이다.
// SPEC-GOOSE-ADAPTER-001 M5 T-063
package factory

import (
	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/deepseek"
	"github.com/modu-ai/goose/internal/llm/provider/ollama"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/provider/xai"
	"go.uber.org/zap"
)

// DefaultRegistryOptions는 NewDefaultRegistry 생성 옵션이다.
type DefaultRegistryOptions struct {
	// SecretStore는 API 키 저장소이다. Ollama는 SecretStore 없이 동작할 수 있다.
	SecretStore provider.SecretStore
	// EnabledProviders는 등록할 provider 이름 목록이다.
	// 빈 값이면 아무 provider도 등록하지 않는다.
	// 지원 목록: "openai", "xai", "deepseek", "ollama" (google은 별도 credential 설정 필요)
	EnabledProviders []string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// NewDefaultRegistry는 opts.EnabledProviders에 지정된 provider들을 등록한
// ProviderRegistry를 생성한다.
//
// Ollama는 credential 없이 동작한다.
// OpenAI, xAI, DeepSeek은 SecretStore와 credential pool이 필요하다.
//
// @MX:NOTE: [AUTO] DefaultRegistry factory — 6개 provider 통합 등록 진입점
func NewDefaultRegistry(opts DefaultRegistryOptions) (*provider.ProviderRegistry, error) {
	reg := provider.NewRegistry()

	for _, name := range opts.EnabledProviders {
		var p provider.Provider

		switch name {
		case "openai":
			if opts.SecretStore == nil {
				continue
			}
			pool := newEmptyPool()
			a, err := openai.New(openai.OpenAIOptions{
				Name:        "openai",
				Pool:        pool,
				SecretStore: opts.SecretStore,
				Capabilities: provider.Capabilities{
					Streaming: true,
					Tools:     true,
					Vision:    true,
				},
				Logger: opts.Logger,
			})
			if err != nil {
				return nil, err
			}
			p = a

		case "xai":
			if opts.SecretStore == nil {
				continue
			}
			pool := newEmptyPool()
			a, err := xai.New(xai.Options{
				Pool:        pool,
				SecretStore: opts.SecretStore,
				Logger:      opts.Logger,
			})
			if err != nil {
				return nil, err
			}
			p = a

		case "deepseek":
			if opts.SecretStore == nil {
				continue
			}
			pool := newEmptyPool()
			a, err := deepseek.New(deepseek.Options{
				Pool:        pool,
				SecretStore: opts.SecretStore,
				Logger:      opts.Logger,
			})
			if err != nil {
				return nil, err
			}
			p = a

		case "ollama":
			// Ollama는 credential 없이 동작
			a, err := ollama.New(ollama.OllamaOptions{
				Logger: opts.Logger,
			})
			if err != nil {
				return nil, err
			}
			p = a

		default:
			// 알 수 없는 provider는 무시
			continue
		}

		if p != nil {
			if err := reg.Register(p); err != nil {
				return nil, err
			}
		}
	}

	return reg, nil
}

// newEmptyPool은 비어있는 credential pool을 생성한다.
// DefaultRegistry에서 provider 인스턴스 생성을 위해 사용된다.
func newEmptyPool() *credential.CredentialPool {
	src := credential.NewDummySource(nil)
	pool, _ := credential.New(src, credential.NewRoundRobinStrategy())
	return pool
}
