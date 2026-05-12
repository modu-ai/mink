// Package deepseek는 DeepSeek API 어댑터를 제공한다.
// OpenAI API 호환 인터페이스를 사용한다.
// SPEC-GOOSE-ADAPTER-001 M2 T-034
package deepseek

import (
	"net/http"

	"github.com/modu-ai/mink/internal/llm/credential"
	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/provider/openai"
	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"go.uber.org/zap"
)

const (
	// deepSeekBaseURL은 DeepSeek API 엔드포인트이다.
	deepSeekBaseURL = "https://api.deepseek.com/v1"
)

// Options는 DeepSeek 어댑터 생성 옵션이다.
type Options struct {
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다. 빈 값이면 기본 클라이언트 사용.
	HTTPClient *http.Client
	// BaseURL은 API 엔드포인트 기본 URL이다. 빈 값이면 deepSeekBaseURL 사용. (테스트 override용)
	BaseURL string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// New는 DeepSeek용 OpenAIAdapter를 생성한다.
// openai.New를 호출하며 BaseURL을 https://api.deepseek.com/v1로 설정한다.
// DeepSeek은 Vision을 지원하지 않으므로 Vision=false로 설정된다.
func New(opts Options) (*openai.OpenAIAdapter, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = deepSeekBaseURL
	}

	return openai.New(openai.OpenAIOptions{
		Name:        "deepseek",
		BaseURL:     baseURL,
		Pool:        opts.Pool,
		Tracker:     opts.Tracker,
		SecretStore: opts.SecretStore,
		HTTPClient:  opts.HTTPClient,
		Capabilities: provider.Capabilities{
			Streaming:        true,
			Tools:            true,
			Vision:           false, // DeepSeek does not support vision
			Embed:            false,
			AdaptiveThinking: false,
			// JSONMode is supported via OpenAI-compat response_format (REQ-AMEND-012).
			// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-012
			JSONMode: true,
			// UserID is undocumented in DeepSeek API — silent drop adopted (REQ-AMEND-012).
			// @MX:SPEC SPEC-GOOSE-ADAPTER-001-AMEND-001 REQ-AMEND-012
			UserID: false,
		},
		Logger: opts.Logger,
	})
}
