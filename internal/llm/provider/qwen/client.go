// Package qwen는 Alibaba DashScope Qwen API 어댑터를 제공한다.
// OpenAI API 호환 인터페이스를 사용하며, 국제판/중국판 지역 선택을 지원한다.
// SPEC-GOOSE-ADAPTER-002 M3
package qwen

import (
	"errors"
	"net/http"
	"os"

	"github.com/modu-ai/goose/internal/llm/credential"
	"github.com/modu-ai/goose/internal/llm/provider"
	"github.com/modu-ai/goose/internal/llm/provider/openai"
	"github.com/modu-ai/goose/internal/llm/ratelimit"
	"go.uber.org/zap"
)

// ErrInvalidRegion은 지원하지 않는 region 값이 설정되었을 때 반환된다.
// REQ-ADP2-018: "intl"과 "cn"만 허용.
var ErrInvalidRegion = errors.New("qwen: invalid region; must be 'intl' or 'cn'")

// Region은 DashScope API 지역이다.
type Region string

const (
	// RegionIntl은 국제판 DashScope 엔드포인트이다 (기본값).
	RegionIntl Region = "intl"
	// RegionCN은 중국판 DashScope 엔드포인트이다.
	RegionCN Region = "cn"
)

const (
	// 국제판 DashScope 엔드포인트 (REQ-ADP2-011).
	dashscopeIntlURL = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
	// 중국판 DashScope 엔드포인트.
	dashscopeCNURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	// GOOSE_QWEN_REGION 환경변수 키.
	envQwenRegion = "GOOSE_QWEN_REGION"
)

// Options는 Qwen 어댑터 생성 옵션이다.
type Options struct {
	// Pool은 credential pool이다.
	Pool *credential.CredentialPool
	// Tracker는 rate limit tracker이다.
	Tracker *ratelimit.Tracker
	// SecretStore는 secret 저장소이다.
	SecretStore provider.SecretStore
	// HTTPClient는 HTTP 요청에 사용할 클라이언트이다. 빈 값이면 기본 클라이언트 사용.
	HTTPClient *http.Client
	// Region은 DashScope API 지역이다.
	// 빈 값이면 GOOSE_QWEN_REGION 환경변수를 참조하고, 없으면 RegionIntl(기본값) 사용.
	// REQ-ADP2-011, REQ-ADP2-018
	Region Region
	// BaseURL은 API 엔드포인트 기본 URL이다. 빈 값이면 Region에 따라 자동 결정. (테스트 override용)
	BaseURL string
	// Logger는 구조화 로거이다.
	Logger *zap.Logger
}

// New는 Qwen DashScope용 OpenAIAdapter를 생성한다.
// Region → GOOSE_QWEN_REGION 환경변수 → intl(기본값) 순으로 URL 결정.
// AC-ADP2-010, AC-ADP2-011, AC-ADP2-012
func New(opts Options) (*openai.OpenAIAdapter, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		resolvedURL, err := resolveBaseURL(string(opts.Region))
		if err != nil {
			return nil, err
		}
		baseURL = resolvedURL
	}

	return openai.New(openai.OpenAIOptions{
		Name:        "qwen",
		BaseURL:     baseURL,
		Pool:        opts.Pool,
		Tracker:     opts.Tracker,
		SecretStore: opts.SecretStore,
		HTTPClient:  opts.HTTPClient,
		Capabilities: provider.Capabilities{
			Streaming:        true,
			Tools:            true,
			Vision:           true, // Qwen-VL 멀티모달 모델 지원
			Embed:            false,
			AdaptiveThinking: false,
			MaxContextTokens: 131072,
			MaxOutputTokens:  8192,
		},
		Logger: opts.Logger,
	})
}

// resolveBaseURL은 Region 문자열로 DashScope BaseURL을 결정한다.
// 빈 region이면 GOOSE_QWEN_REGION 환경변수를 참조하고, 없으면 intl을 사용한다.
// "intl"과 "cn" 외의 값은 ErrInvalidRegion을 반환한다 (REQ-ADP2-018).
func resolveBaseURL(region string) (string, error) {
	if region == "" {
		region = os.Getenv(envQwenRegion)
	}
	if region == "" {
		region = string(RegionIntl)
	}

	switch Region(region) {
	case RegionIntl:
		return dashscopeIntlURL, nil
	case RegionCN:
		return dashscopeCNURL, nil
	default:
		return "", ErrInvalidRegion
	}
}
