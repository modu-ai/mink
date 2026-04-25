// Package openrouter는 OpenRouter gateway 어댑터를 제공한다.
// OpenAI API 호환 인터페이스를 사용하며, ranking 헤더(HTTP-Referer, X-Title) 주입과
// PreferredProviders 라우팅을 지원한다.
// SPEC-GOOSE-ADAPTER-002 M2 + OI-1 (v0.3)
package openrouter

import (
	"context"
	"net/http"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"github.com/modu-ai/goose/internal/message"
	"go.uber.org/zap"
)

const (
	// openRouterBaseURL은 OpenRouter API 엔드포인트이다.
	openRouterBaseURL = "https://openrouter.ai/api/v1"
)

// Options는 OpenRouter 어댑터 생성 옵션이다.
type Options struct {
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다. 빈 값이면 기본 클라이언트 사용.
	HTTPClient *http.Client
	// BaseURL은 API 엔드포인트 기본 URL이다. 빈 값이면 openRouterBaseURL 사용. (테스트 override용)
	BaseURL string
	// HTTPReferer는 OpenRouter 모델 ranking에 영향을 주는 HTTP-Referer 헤더 값이다 (선택).
	// 빈 값이면 헤더를 주입하지 않는다.
	HTTPReferer string
	// XTitle는 OpenRouter 모델 ranking에 영향을 주는 X-Title 헤더 값이다 (선택).
	// 빈 값이면 헤더를 주입하지 않는다.
	XTitle string
	// PreferredProviders는 upstream provider routing 우선순위이다 (선택).
	// 비어있지 않으면 request body에 `"provider": {"order":[...], "allow_fallbacks":true}`로 주입된다.
	// REQ-ADP2-020 (OI-1 v0.3).
	PreferredProviders []string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// Adapter는 OpenRouter OpenAIAdapter 래퍼이다.
// openai.OpenAIAdapter를 embedding하여 Provider 인터페이스를 상속하고,
// Stream/Complete를 override하여 PreferredProviders를 ExtraRequestFields에 주입한다.
type Adapter struct {
	*openai.OpenAIAdapter
	preferredProviders []string
}

// New는 OpenRouter용 Adapter를 생성한다.
// REQ-ADP2-008: HTTPReferer/XTitle 비어있지 않으면 ExtraHeaders로 주입.
// REQ-ADP2-020 (OI-1): PreferredProviders 비어있지 않으면 Stream/Complete에서 주입.
// AC-ADP2-005: HTTP-Referer, X-Title 헤더 검증.
func New(opts Options) (*Adapter, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = openRouterBaseURL
	}

	// ranking 헤더 구성: 비어있는 값은 주입하지 않음 (REQ-ADP2-008)
	var extraHeaders map[string]string
	if opts.HTTPReferer != "" || opts.XTitle != "" {
		extraHeaders = make(map[string]string, 2)
		if opts.HTTPReferer != "" {
			extraHeaders["HTTP-Referer"] = opts.HTTPReferer
		}
		if opts.XTitle != "" {
			extraHeaders["X-Title"] = opts.XTitle
		}
	}

	inner, err := openai.New(openai.OpenAIOptions{
		Name:         "openrouter",
		BaseURL:      baseURL,
		Pool:         opts.Pool,
		Tracker:      opts.Tracker,
		SecretStore:  opts.SecretStore,
		HTTPClient:   opts.HTTPClient,
		ExtraHeaders: extraHeaders,
		Capabilities: provider.Capabilities{
			Streaming:        true,
			Tools:            true,
			Vision:           true, // 300+ 모델 gateway: vision 모델 포함
			Embed:            false,
			AdaptiveThinking: false,
			MaxContextTokens: 200000,
			MaxOutputTokens:  16384,
		},
		Logger: opts.Logger,
	})
	if err != nil {
		return nil, err
	}

	// PreferredProviders 복제: 호출자가 이후 slice를 mutate해도 어댑터에 영향 없음
	var pp []string
	if len(opts.PreferredProviders) > 0 {
		pp = make([]string, len(opts.PreferredProviders))
		copy(pp, opts.PreferredProviders)
	}

	return &Adapter{
		OpenAIAdapter:      inner,
		preferredProviders: pp,
	}, nil
}

// Name은 provider 이름을 반환한다.
func (a *Adapter) Name() string { return "openrouter" }

// Stream은 PreferredProviders를 ExtraRequestFields에 merge한 후 openai.Stream에 위임한다.
// REQ-ADP2-020 (OI-1 v0.3).
func (a *Adapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	a.injectProviderRouting(&req)
	return a.OpenAIAdapter.Stream(ctx, req)
}

// Complete은 PreferredProviders를 적용한 후 openai.Complete에 위임한다.
func (a *Adapter) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	a.injectProviderRouting(&req)
	return a.OpenAIAdapter.Complete(ctx, req)
}

// injectProviderRouting은 preferredProviders를 ExtraRequestFields에 merge한다.
// 호출자 map 보호를 위해 deep-copy 후 mutate한다.
// 빈 PreferredProviders이면 no-op.
func (a *Adapter) injectProviderRouting(req *provider.CompletionRequest) {
	if len(a.preferredProviders) == 0 {
		return
	}
	// 호출자 map 보호: 새로운 map 할당 + 기존 항목 복사 + provider 키 추가
	newExtra := make(map[string]any, len(req.ExtraRequestFields)+1)
	for k, v := range req.ExtraRequestFields {
		newExtra[k] = v
	}
	// providers slice 복제 (호출자 mutation 방지 + 어댑터 내부 격리)
	order := make([]string, len(a.preferredProviders))
	copy(order, a.preferredProviders)
	newExtra["provider"] = map[string]any{
		"order":           order,
		"allow_fallbacks": true,
	}
	req.ExtraRequestFields = newExtra
}

// Ensure Adapter implements provider.Provider at compile time.
var _ provider.Provider = (*Adapter)(nil)
