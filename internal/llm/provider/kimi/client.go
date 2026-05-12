// Package kimi는 Moonshot AI Kimi API 어댑터를 제공한다.
// OpenAI API 호환 인터페이스를 사용하며, 국제판/중국판 지역 선택을 지원한다.
// OpenAI-compat endpoint만 지원 (Anthropic-compat은 별도 SPEC).
// SPEC-GOOSE-ADAPTER-002 M3
package kimi

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/modu-ai/mink/internal/envalias"
	"github.com/modu-ai/mink/internal/llm/credential"
	"github.com/modu-ai/mink/internal/llm/provider"
	"github.com/modu-ai/mink/internal/llm/provider/openai"
	"github.com/modu-ai/mink/internal/llm/ratelimit"
	"github.com/modu-ai/mink/internal/message"
)

// ErrInvalidRegion은 지원하지 않는 region 값이 설정되었을 때 반환된다.
var ErrInvalidRegion = errors.New("kimi: invalid region; must be 'intl' or 'cn'")

// Region은 Moonshot AI API 지역이다.
type Region string

const (
	// RegionIntl은 국제판 Moonshot AI 엔드포인트이다 (기본값).
	RegionIntl Region = "intl"
	// RegionCN은 중국판 Moonshot AI 엔드포인트이다.
	RegionCN Region = "cn"
)

const (
	// 국제판 Moonshot AI 엔드포인트 (REQ-ADP2-012).
	kimiIntlURL = "https://api.moonshot.ai/v1"
	// 중국판 Moonshot AI 엔드포인트.
	kimiCNURL = "https://api.moonshot.cn/v1"
	// envKimiRegion은 alias loader short key 이다 (SPEC-MINK-ENV-MIGRATE-001 §4.5 OQ-PL-2).
	// 값이 short key "KIMI_REGION" 으로 변경됨: DefaultGet("KIMI_REGION") →
	// MINK_KIMI_REGION (primary) / GOOSE_KIMI_REGION (legacy alias).
	envKimiRegion = "KIMI_REGION"
)

// Options는 Kimi 어댑터 생성 옵션이다.
type Options struct {
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다. 빈 값이면 기본 클라이언트 사용.
	HTTPClient *http.Client
	// Region은 Moonshot AI API 지역이다.
	// 빈 값이면 GOOSE_KIMI_REGION 환경변수를 참조하고, 없으면 RegionIntl(기본값) 사용.
	// REQ-ADP2-012
	Region Region
	// BaseURL은 API 엔드포인트 기본 URL이다. 빈 값이면 Region에 따라 자동 결정. (테스트 override용)
	BaseURL string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// Adapter는 Kimi Moonshot OpenAIAdapter 래퍼이다.
// openai.OpenAIAdapter를 embedding하여 Provider 인터페이스를 상속하고,
// Stream/Complete를 override하여 long-context advisory(OI-3, REQ-ADP2-022)를 주입한다.
type Adapter struct {
	*openai.OpenAIAdapter
	logger *zap.Logger
}

// New는 Kimi Moonshot AI용 Adapter를 생성한다.
// Region → GOOSE_KIMI_REGION 환경변수 → intl(기본값) 순으로 URL 결정.
// AC-ADP2-013, AC-ADP2-014
func New(opts Options) (*Adapter, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		resolvedURL, err := resolveBaseURL(string(opts.Region))
		if err != nil {
			return nil, err
		}
		baseURL = resolvedURL
	}

	inner, err := openai.New(openai.OpenAIOptions{
		Name:        "kimi",
		BaseURL:     baseURL,
		Pool:        opts.Pool,
		Tracker:     opts.Tracker,
		SecretStore: opts.SecretStore,
		HTTPClient:  opts.HTTPClient,
		Capabilities: provider.Capabilities{
			Streaming:        true,
			Tools:            true,
			Vision:           true, // Kimi K2.6 1T MoE: 멀티모달 지원
			Embed:            false,
			AdaptiveThinking: false,
			MaxContextTokens: 262144, // K2.6 262K context
			MaxOutputTokens:  98304,  // K2.6 98K max output
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
func (a *Adapter) Name() string { return "kimi" }

// Stream은 long-context advisory를 INFO 로깅한 후 openai.Stream에 위임한다.
// REQ-ADP2-022 (OI-3 v0.3): moonshot-v1-128k 클래스 모델 + 64K 초과 입력 시 INFO 1건.
func (a *Adapter) Stream(ctx context.Context, req provider.CompletionRequest) (<-chan message.StreamEvent, error) {
	applyAdvisory(a.logger, req)
	return a.OpenAIAdapter.Stream(ctx, req)
}

// Complete은 long-context advisory를 INFO 로깅한 후 openai.Complete에 위임한다.
func (a *Adapter) Complete(ctx context.Context, req provider.CompletionRequest) (*provider.CompletionResponse, error) {
	applyAdvisory(a.logger, req)
	return a.OpenAIAdapter.Complete(ctx, req)
}

// Ensure Adapter implements provider.Provider at compile time.
var _ provider.Provider = (*Adapter)(nil)

// resolveBaseURL은 Region 문자열로 Moonshot AI BaseURL을 결정한다.
// 빈 region이면 GOOSE_KIMI_REGION 환경변수를 참조하고, 없으면 intl을 사용한다.
// "intl"과 "cn" 외의 값은 ErrInvalidRegion을 반환한다.
func resolveBaseURL(region string) (string, error) {
	if region == "" {
		region, _, _ = envalias.DefaultGet(envKimiRegion)
	}
	if region == "" {
		region = string(RegionIntl)
	}

	switch Region(region) {
	case RegionIntl:
		return kimiIntlURL, nil
	case RegionCN:
		return kimiCNURL, nil
	default:
		return "", ErrInvalidRegion
	}
}
