package ratelimit

import "time"

// openRouterParser는 OpenRouter x-ratelimit-* 헤더를 파싱한다.
// §6.5: OpenAI와 동일 포맷이므로 OpenAI 파서를 래핑.
type openRouterParser struct {
	inner Parser
}

// NewOpenRouterParser는 OpenRouter 헤더 파서를 생성한다.
// 내부적으로 OpenAI 파서를 재사용하고 provider 이름만 "openrouter"로 반환한다.
func NewOpenRouterParser() Parser {
	return &openRouterParser{
		inner: NewOpenAIParser("openrouter"),
	}
}

func (p *openRouterParser) Provider() string { return "openrouter" }

func (p *openRouterParser) Parse(headers map[string]string, now time.Time) (RateLimitState, []string) {
	return p.inner.Parse(headers, now)
}
