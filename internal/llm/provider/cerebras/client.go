// Package cerebras는 Cerebras API 어댑터를 제공한다.
// OpenAI API 호환 인터페이스를 사용한다.
// SPEC-GOOSE-ADAPTER-002 M1
package cerebras

import (
	"net/http"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"go.uber.org/zap"
)

const (
	// cerebrasBaseURL은 Cerebras API 엔드포인트이다 (OpenAI-compat).
	cerebrasBaseURL = "https://api.cerebras.ai/v1"
)

// Options는 Cerebras 어댑터 생성 옵션이다.
type Options struct {
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다. 빈 값이면 기본 클라이언트 사용.
	HTTPClient *http.Client
	// BaseURL은 API 엔드포인트 기본 URL이다. 빈 값이면 cerebrasBaseURL 사용. (테스트 override용)
	BaseURL string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// New는 Cerebras용 OpenAIAdapter를 생성한다.
// openai.New를 호출하며 BaseURL을 https://api.cerebras.ai/v1로 설정한다.
// AC-ADP2-008: Cerebras streaming (1000+ TPS Wafer-Scale Engine).
func New(opts Options) (*openai.OpenAIAdapter, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = cerebrasBaseURL
	}

	return openai.New(openai.OpenAIOptions{
		Name:        "cerebras",
		BaseURL:     baseURL,
		Pool:        opts.Pool,
		Tracker:     opts.Tracker,
		SecretStore: opts.SecretStore,
		HTTPClient:  opts.HTTPClient,
		Capabilities: provider.Capabilities{
			Streaming:        true,
			Tools:            true,
			Vision:           false, // Cerebras Wafer-Scale: text-only, vision 미지원
			Embed:            false,
			AdaptiveThinking: false,
			MaxContextTokens: 128000,
			MaxOutputTokens:  16384,
		},
		Logger: opts.Logger,
	})
}
