// Package together는 Together AI API 어댑터를 제공한다.
// OpenAI API 호환 인터페이스를 사용한다.
// SPEC-GOOSE-ADAPTER-002 M2
package together

import (
	"net/http"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"go.uber.org/zap"
)

const (
	// togetherBaseURL은 Together AI API 엔드포인트이다 (OpenAI-compat).
	togetherBaseURL = "https://api.together.xyz/v1"
)

// Options는 Together AI 어댑터 생성 옵션이다.
type Options struct {
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다. 빈 값이면 기본 클라이언트 사용.
	HTTPClient *http.Client
	// BaseURL은 API 엔드포인트 기본 URL이다. 빈 값이면 togetherBaseURL 사용. (테스트 override용)
	BaseURL string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// New는 Together AI용 OpenAIAdapter를 생성한다.
// openai.New를 호출하며 BaseURL을 https://api.together.xyz/v1로 설정한다.
// AC-ADP2-006: Together streaming (173 모델, Llama/Qwen/Mixtral 등).
func New(opts Options) (*openai.OpenAIAdapter, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = togetherBaseURL
	}

	return openai.New(openai.OpenAIOptions{
		Name:        "together",
		BaseURL:     baseURL,
		Pool:        opts.Pool,
		Tracker:     opts.Tracker,
		SecretStore: opts.SecretStore,
		HTTPClient:  opts.HTTPClient,
		Capabilities: provider.Capabilities{
			Streaming:        true,
			Tools:            true,
			Vision:           true,  // Together: Llama 3.2 Vision 등 multimodal 모델 포함
			Embed:            true,  // M2-BERT 등 임베딩 모델 지원
			AdaptiveThinking: false,
			MaxContextTokens: 128000,
			MaxOutputTokens:  32768,
		},
		Logger: opts.Logger,
	})
}
