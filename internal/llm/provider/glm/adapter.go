// Package glm는 Z.ai GLM API 어댑터를 제공한다.
// openai.OpenAIAdapter를 embedding하고 Stream을 override하여 thinking 파라미터를 주입한다.
// SPEC-GOOSE-ADAPTER-002 M4
package glm

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
	// glmBaseURL은 Z.ai GLM API 공식 엔드포인트이다 (REQ-ADP2-022, 구 bigmodel.cn 이전).
	glmBaseURL = "https://api.z.ai/api/paas/v4"
)

// Adapter는 Z.ai GLM OpenAIAdapter 래퍼이다.
// openai.OpenAIAdapter를 embedding하여 Provider 인터페이스를 상속하고,
// Stream/Complete를 override하여 thinking 파라미터를 주입한다.
//
// @MX:ANCHOR: [AUTO] GLM thinking mode injection — Provider 인터페이스 구현 진입점
// @MX:REASON: Stream override + ExtraRequestFields mutation이 GLM 전용 로직을 캡슐화, fan_in >= 3 예상
type Adapter struct {
	*openai.OpenAIAdapter
	logger *zap.Logger
}

// Options는 GLM 어댑터 생성 옵션이다.
type Options struct {
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다. 빈 값이면 기본 클라이언트 사용.
	HTTPClient *http.Client
	// BaseURL은 API 엔드포인트 기본 URL이다. 빈 값이면 glmBaseURL 사용. (테스트 override용)
	BaseURL string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// New는 GLM용 Adapter를 생성한다.
// openai.OpenAIAdapter를 embedding하고 thinking mode 지원을 추가한다.
// AC-ADP2-001, AC-ADP2-002, AC-ADP2-003
func New(opts Options) (*Adapter, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = glmBaseURL
	}

	inner, err := openai.New(openai.OpenAIOptions{
		Name:        "glm",
		BaseURL:     baseURL,
		Pool:        opts.Pool,
		Tracker:     opts.Tracker,
		SecretStore: opts.SecretStore,
		HTTPClient:  opts.HTTPClient,
		Capabilities: provider.Capabilities{
			Streaming:        true,
			Tools:            true,
			Vision:           true,  // GLM-4.6+ 멀티모달 지원
			Embed:            false,
			AdaptiveThinking: true, // GLM thinking mode 지원 (glm-4.5, 4.6, 4.7, 5)
			MaxContextTokens: 200000,
			MaxOutputTokens:  131072,
		},
		Logger: opts.Logger,
	})
	if err != nil {
		return nil, err
	}

	return &Adapter{
		OpenAIAdapter: inner,
		logger:        opts.Logger,
	}, nil
}

// Name은 provider 이름을 반환한다.
func (a *Adapter) Name() string { return "glm" }

// Stream은 thinking 파라미터를 ExtraRequestFields에 merge한 후 openai.Stream에 위임한다.
// REQ-ADP2-007: thinking-capable 모델 → thinking:{type:enabled} 주입.
// REQ-ADP2-014: 미지원 모델 → WARN + 무시 (에러 없음).
//
// @MX:WARN: [AUTO] ExtraRequestFields 복제 후 mutation — 호출자 map 보호
// @MX:REASON: req.ExtraRequestFields는 호출자 소유. deep-copy 없이 mutate하면 레이스 컨디션 위험.
func (a *Adapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	field, ok, reason := BuildThinkingField(req.Thinking, req.Route.Model)
	if !ok {
		// REQ-ADP2-014 graceful degradation: WARN 로그 + thinking 주입 생략
		if a.logger != nil {
			a.logger.Warn("glm.thinking.ignored", zap.String("reason", reason))
		}
	} else if field != nil {
		// 호출자 map 보호를 위해 복제 후 merge
		newExtra := make(map[string]any, len(req.ExtraRequestFields)+len(field))
		for k, v := range req.ExtraRequestFields {
			newExtra[k] = v
		}
		for k, v := range field {
			newExtra[k] = v
		}
		req.ExtraRequestFields = newExtra
	}

	return a.OpenAIAdapter.Stream(ctx, req)
}

// Complete는 thinking 파라미터를 적용한 후 openai.Complete에 위임한다.
func (a *Adapter) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	field, ok, reason := BuildThinkingField(req.Thinking, req.Route.Model)
	if !ok {
		if a.logger != nil {
			a.logger.Warn("glm.thinking.ignored", zap.String("reason", reason))
		}
	} else if field != nil {
		newExtra := make(map[string]any, len(req.ExtraRequestFields)+len(field))
		for k, v := range req.ExtraRequestFields {
			newExtra[k] = v
		}
		for k, v := range field {
			newExtra[k] = v
		}
		req.ExtraRequestFields = newExtra
	}

	return a.OpenAIAdapter.Complete(ctx, req)
}

// Ensure Adapter implements provider.Provider at compile time.
var _ provider.Provider = (*Adapter)(nil)
