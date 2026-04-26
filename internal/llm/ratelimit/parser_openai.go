package ratelimit

import "time"

// openAIParser는 OpenAI(및 호환 provider: xAI, DeepSeek, Groq, OpenRouter) 헤더를 파싱한다.
// §6.3 헤더 매핑.
type openAIParser struct {
	provider string
}

// NewOpenAIParser는 OpenAI 헤더 파서를 생성한다.
// providerName은 "openai", "openrouter", "xai" 등을 수용한다.
// @MX:ANCHOR: [AUTO] OpenAI + compat 계열 parser 팩토리 — openai/openrouter/xai/deepseek/groq 공용
// @MX:REASON: SPEC-GOOSE-RATELIMIT-001 §3.1 "OpenAI-compat 재사용" 정책; fan_in >= 3 예상 (openai, openrouter, alias 등록)
// @MX:SPEC: SPEC-GOOSE-RATELIMIT-001
func NewOpenAIParser(providerName string) Parser {
	return &openAIParser{provider: providerName}
}

func (p *openAIParser) Provider() string { return p.provider }

// Parse는 OpenAI x-ratelimit-* 헤더를 파싱한다.
// Hour 버킷은 OpenAI가 반환하지 않으므로 zero-value 유지(§6.3).
// 파싱 실패 시 해당 버킷을 zero-value로 남기고 debug 메시지 수집(REQ-RL-006).
func (p *openAIParser) Parse(headers map[string]string, now time.Time) (RateLimitState, []string) {
	var debugMsgs []string
	state := RateLimitState{
		Provider:   p.provider,
		CapturedAt: now,
	}

	// requests_min
	reqMin := RateLimitBucket{CapturedAt: now}
	if v, ok := CaseInsensitiveGet(headers, "x-ratelimit-limit-requests"); ok {
		if n, msg := parseIntOrZero(v); msg != "" {
			debugMsgs = append(debugMsgs, msg)
			reqMin = RateLimitBucket{CapturedAt: now}
		} else {
			reqMin.Limit = n
			if r, ok2 := CaseInsensitiveGet(headers, "x-ratelimit-remaining-requests"); ok2 {
				if rn, m := parseIntOrZero(r); m != "" {
					debugMsgs = append(debugMsgs, m)
					reqMin = RateLimitBucket{CapturedAt: now}
				} else {
					reqMin.Remaining = rn
					if rs, ok3 := CaseInsensitiveGet(headers, "x-ratelimit-reset-requests"); ok3 {
						if secs, m := parseDurationSeconds(rs); m != "" {
							debugMsgs = append(debugMsgs, m)
						} else {
							reqMin.ResetSeconds = secs
						}
					}
				}
			}
		}
	}
	state.RequestsMin = reqMin

	// tokens_min
	tokMin := RateLimitBucket{CapturedAt: now}
	if v, ok := CaseInsensitiveGet(headers, "x-ratelimit-limit-tokens"); ok {
		if n, msg := parseIntOrZero(v); msg != "" {
			debugMsgs = append(debugMsgs, msg)
			tokMin = RateLimitBucket{CapturedAt: now}
		} else {
			tokMin.Limit = n
			if r, ok2 := CaseInsensitiveGet(headers, "x-ratelimit-remaining-tokens"); ok2 {
				if rn, m := parseIntOrZero(r); m != "" {
					debugMsgs = append(debugMsgs, m)
					tokMin = RateLimitBucket{CapturedAt: now}
				} else {
					tokMin.Remaining = rn
					if rs, ok3 := CaseInsensitiveGet(headers, "x-ratelimit-reset-tokens"); ok3 {
						if secs, m := parseDurationSeconds(rs); m != "" {
							debugMsgs = append(debugMsgs, m)
						} else {
							tokMin.ResetSeconds = secs
						}
					}
				}
			}
		}
	}
	state.TokensMin = tokMin

	// Hour 버킷: OpenAI는 반환 안 함 → zero-value
	state.RequestsHour = RateLimitBucket{CapturedAt: now}
	state.TokensHour = RateLimitBucket{CapturedAt: now}

	return state, debugMsgs
}
