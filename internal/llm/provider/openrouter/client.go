// Package openrouter는 OpenRouter gateway 어댑터를 제공한다.
// OpenAI API 호환 인터페이스를 사용하며, ranking 헤더(HTTP-Referer, X-Title) 주입을 지원한다.
// SPEC-GOOSE-ADAPTER-002 M2
package openrouter

import (
	"net/http"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
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
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// New는 OpenRouter용 OpenAIAdapter를 생성한다.
// REQ-ADP2-008: HTTPReferer/XTitle 비어있지 않으면 ExtraHeaders로 주입.
// AC-ADP2-005: HTTP-Referer, X-Title 헤더 검증.
func New(opts Options) (*openai.OpenAIAdapter, error) {
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

	return openai.New(openai.OpenAIOptions{
		Name:        "openrouter",
		BaseURL:     baseURL,
		Pool:        opts.Pool,
		Tracker:     opts.Tracker,
		SecretStore: opts.SecretStore,
		HTTPClient:  opts.HTTPClient,
		ExtraHeaders: extraHeaders,
		Capabilities: provider.Capabilities{
			Streaming:        true,
			Tools:            true,
			Vision:           true,  // 300+ 모델 gateway: vision 모델 포함
			Embed:            false,
			AdaptiveThinking: false,
			MaxContextTokens: 200000,
			MaxOutputTokens:  16384,
		},
		Logger: opts.Logger,
	})
}
