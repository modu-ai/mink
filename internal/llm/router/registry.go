package router

import (
	"errors"
	"sort"
)

// ProviderMeta는 LLM provider의 메타데이터이다.
type ProviderMeta struct {
	// Name은 provider의 고유 식별자이다 (소문자, 예: "anthropic").
	Name string
	// DisplayName은 사람이 읽기 쉬운 provider 이름이다.
	DisplayName string
	// DefaultBaseURL은 provider API의 기본 base URL이다.
	DefaultBaseURL string
	// AuthType은 인증 방식이다 ("oauth" | "api_key" | "none").
	AuthType string
	// SupportsStream은 스트리밍 응답 지원 여부이다.
	SupportsStream bool
	// SupportsTools는 function/tool calling 지원 여부이다.
	SupportsTools bool
	// SupportsVision은 이미지/비전 입력 지원 여부이다.
	SupportsVision bool
	// SupportsEmbed는 임베딩 생성 지원 여부이다.
	SupportsEmbed bool
	// AdapterReady는 Phase 1 ADAPTER-001에서 실제 HTTP 호출이 구현되었는지 여부이다.
	AdapterReady bool
	// SuggestedModels는 이 provider에서 권장하는 모델 이름 목록이다.
	SuggestedModels []string
}

// ProviderRegistry는 LLM provider 메타데이터의 레지스트리이다.
//
// 생성 시점 이후 providers 맵은 read-only로 사용되므로 mutex가 필요 없다.
// Register는 생성 시점에만 호출되어야 하며, 동시 호출을 지원하지 않는다.
type ProviderRegistry struct {
	providers map[string]*ProviderMeta
	// order는 List()의 결정적 순서를 위한 이름 슬라이스이다.
	order []string
}

// NewRegistry는 빈 ProviderRegistry를 생성한다.
func NewRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]*ProviderMeta),
	}
}

// Register는 provider 메타데이터를 레지스트리에 등록한다.
//
// 동일 이름의 provider가 이미 등록되어 있으면 에러를 반환한다.
// AuthType은 "oauth", "api_key", "none" 중 하나여야 한다.
// Name은 비어 있으면 안 된다.
// Name이 "custom"이 아닌 경우 DefaultBaseURL은 필수이다.
func (r *ProviderRegistry) Register(meta *ProviderMeta) error {
	if meta.Name == "" {
		return errors.New("registry: provider name is required")
	}
	if meta.Name != "custom" && meta.DefaultBaseURL == "" {
		return errors.New("registry: DefaultBaseURL is required for non-custom providers")
	}
	if meta.AuthType != "oauth" && meta.AuthType != "api_key" && meta.AuthType != "none" {
		return errors.New("registry: invalid auth_type; must be oauth, api_key, or none")
	}
	if _, exists := r.providers[meta.Name]; exists {
		return errors.New("registry: provider " + meta.Name + " already registered")
	}
	r.providers[meta.Name] = meta
	r.order = append(r.order, meta.Name)
	// 이름 알파벳 순으로 정렬하여 결정적 순서 유지
	sort.Strings(r.order)
	return nil
}

// Get은 이름으로 provider 메타데이터를 조회한다.
// 등록되지 않은 provider이면 false를 반환한다.
func (r *ProviderRegistry) Get(name string) (*ProviderMeta, bool) {
	p, ok := r.providers[name]
	return p, ok
}

// List는 등록된 모든 provider 메타데이터를 이름 알파벳 순으로 반환한다.
// @MX:ANCHOR: [AUTO] provider 목록 열거 공개 API — 레지스트리 전체 조회 진입점
// @MX:REASON: DefaultRegistry 검증 테스트 및 라우팅 결정에서 fan_in >= 3 예상
func (r *ProviderRegistry) List() []*ProviderMeta {
	result := make([]*ProviderMeta, 0, len(r.order))
	for _, name := range r.order {
		if p, ok := r.providers[name]; ok {
			result = append(result, p)
		}
	}
	return result
}

// DefaultRegistry는 Phase 1에서 지원하는 15+ provider를 사전 등록한 레지스트리를 반환한다.
//
// AdapterReady=true (Phase 1 ADAPTER-001 구현 대상): Anthropic, OpenAI, Google Gemini, xAI, DeepSeek, Ollama
// AdapterReady=false (metadata-only): OpenRouter, Nous, Mistral, Groq, Qwen, Kimi, GLM, MiniMax, Cohere
// @MX:ANCHOR: [AUTO] 기본 레지스트리 생성 진입점 — 라우터 초기화 시 단일 진입점
// @MX:REASON: Router.New(), 통합 테스트, 각 도메인 코드에서 호출 — fan_in >= 3
func DefaultRegistry() *ProviderRegistry {
	reg := NewRegistry()

	// Phase 1 adapter-ready providers (6종)
	mustRegister(reg, &ProviderMeta{
		Name:            "anthropic",
		DisplayName:     "Anthropic",
		DefaultBaseURL:  "https://api.anthropic.com/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  true,
		SupportsEmbed:   false,
		AdapterReady:    true,
		SuggestedModels: []string{"claude-opus-4-7", "claude-sonnet-4-6", "claude-haiku-4"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "openai",
		DisplayName:     "OpenAI",
		DefaultBaseURL:  "https://api.openai.com/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  true,
		SupportsEmbed:   true,
		AdapterReady:    true,
		SuggestedModels: []string{"gpt-4o", "gpt-4o-mini", "o1-preview"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "google",
		DisplayName:     "Google Gemini",
		DefaultBaseURL:  "https://generativelanguage.googleapis.com/v1beta",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  true,
		SupportsEmbed:   true,
		AdapterReady:    true,
		SuggestedModels: []string{"gemini-2.0-flash", "gemini-1.5-pro"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "xai",
		DisplayName:     "xAI",
		DefaultBaseURL:  "https://api.x.ai/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  true,
		SupportsEmbed:   false,
		AdapterReady:    true,
		SuggestedModels: []string{"grok-2", "grok-2-vision"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "deepseek",
		DisplayName:     "DeepSeek",
		DefaultBaseURL:  "https://api.deepseek.com/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  false,
		SupportsEmbed:   false,
		AdapterReady:    true,
		SuggestedModels: []string{"deepseek-chat", "deepseek-coder"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "ollama",
		DisplayName:     "Ollama",
		DefaultBaseURL:  "http://localhost:11434",
		AuthType:        "none",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  true,
		SupportsEmbed:   true,
		AdapterReady:    true,
		SuggestedModels: []string{"llama3.2", "qwen2.5", "phi4"},
	})

	// Phase 1 metadata-only providers (9종)
	mustRegister(reg, &ProviderMeta{
		Name:            "cohere",
		DisplayName:     "Cohere",
		DefaultBaseURL:  "https://api.cohere.ai/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  false,
		SupportsEmbed:   true,
		AdapterReady:    false,
		SuggestedModels: []string{"command-r-plus", "command-r"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "glm",
		DisplayName:     "GLM (ZhipuAI)",
		DefaultBaseURL:  "https://open.bigmodel.cn/api/paas/v4",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  true,
		SupportsEmbed:   false,
		AdapterReady:    false,
		SuggestedModels: []string{"glm-4", "glm-4-flash"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "groq",
		DisplayName:     "Groq",
		DefaultBaseURL:  "https://api.groq.com/openai/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  false,
		SupportsEmbed:   false,
		AdapterReady:    false,
		SuggestedModels: []string{"llama3.2-70b", "mixtral-8x7b"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "kimi",
		DisplayName:     "Kimi (Moonshot)",
		DefaultBaseURL:  "https://api.moonshot.cn/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  false,
		SupportsEmbed:   false,
		AdapterReady:    false,
		SuggestedModels: []string{"moonshot-v1-8k", "moonshot-v1-32k"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "minimax",
		DisplayName:     "MiniMax",
		DefaultBaseURL:  "https://api.minimax.chat/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   false,
		SupportsVision:  false,
		SupportsEmbed:   false,
		AdapterReady:    false,
		SuggestedModels: []string{"abab6", "abab5.5"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "mistral",
		DisplayName:     "Mistral AI",
		DefaultBaseURL:  "https://api.mistral.ai/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  false,
		SupportsEmbed:   true,
		AdapterReady:    false,
		SuggestedModels: []string{"mistral-large", "mistral-small"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "nous",
		DisplayName:     "Nous Research",
		DefaultBaseURL:  "https://inference.nomic.ai/v1",
		AuthType:        "oauth",
		SupportsStream:  true,
		SupportsTools:   false,
		SupportsVision:  false,
		SupportsEmbed:   false,
		AdapterReady:    false,
		SuggestedModels: []string{"hermes-3", "hermes-2-pro"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "openrouter",
		DisplayName:     "OpenRouter",
		DefaultBaseURL:  "https://openrouter.ai/api/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  true,
		SupportsEmbed:   false,
		AdapterReady:    false,
		SuggestedModels: []string{"openai/gpt-4o", "anthropic/claude-3-5-sonnet"},
	})

	mustRegister(reg, &ProviderMeta{
		Name:            "qwen",
		DisplayName:     "Qwen (Alibaba)",
		DefaultBaseURL:  "https://dashscope.aliyuncs.com/compatible-mode/v1",
		AuthType:        "api_key",
		SupportsStream:  true,
		SupportsTools:   true,
		SupportsVision:  true,
		SupportsEmbed:   false,
		AdapterReady:    false,
		SuggestedModels: []string{"qwen3", "qwen2.5-72b"},
	})

	return reg
}

// mustRegister는 Register를 호출하고 실패하면 패닉한다.
// DefaultRegistry 초기화 시에만 사용한다 (프로그래밍 오류).
func mustRegister(reg *ProviderRegistry, meta *ProviderMeta) {
	if err := reg.Register(meta); err != nil {
		panic("router: DefaultRegistry 초기화 실패: " + err.Error())
	}
}
