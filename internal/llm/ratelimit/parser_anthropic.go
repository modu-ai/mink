package ratelimit

import "time"

// anthropicParser는 Anthropic anthropic-ratelimit-* 헤더를 파싱한다.
// §6.4: ISO 8601 reset timestamp 정규화.
type anthropicParser struct{}

// NewAnthropicParser는 Anthropic 헤더 파서를 생성한다.
func NewAnthropicParser() Parser {
	return &anthropicParser{}
}

func (p *anthropicParser) Provider() string { return "anthropic" }

// Parse는 Anthropic anthropic-ratelimit-* 헤더를 파싱한다.
// reset 값은 ISO 8601 timestamp → (reset - now).Seconds() 변환(REQ-RL-004, AC-RL-004).
func (p *anthropicParser) Parse(headers map[string]string, now time.Time) (RateLimitState, []string) {
	var debugMsgs []string
	state := RateLimitState{
		Provider:   "anthropic",
		CapturedAt: now,
	}

	// requests_min
	reqMin := RateLimitBucket{CapturedAt: now}
	if v, ok := CaseInsensitiveGet(headers, "anthropic-ratelimit-requests-limit"); ok {
		if n, msg := parseIntOrZero(v); msg != "" {
			debugMsgs = append(debugMsgs, msg)
			reqMin = RateLimitBucket{CapturedAt: now}
		} else {
			reqMin.Limit = n
			if r, ok2 := CaseInsensitiveGet(headers, "anthropic-ratelimit-requests-remaining"); ok2 {
				if rn, m := parseIntOrZero(r); m != "" {
					debugMsgs = append(debugMsgs, m)
					reqMin = RateLimitBucket{CapturedAt: now}
				} else {
					reqMin.Remaining = rn
					if rs, ok3 := CaseInsensitiveGet(headers, "anthropic-ratelimit-requests-reset"); ok3 {
						if secs, m := parseISO8601ResetSeconds(rs, now); m != "" {
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
	if v, ok := CaseInsensitiveGet(headers, "anthropic-ratelimit-tokens-limit"); ok {
		if n, msg := parseIntOrZero(v); msg != "" {
			debugMsgs = append(debugMsgs, msg)
			tokMin = RateLimitBucket{CapturedAt: now}
		} else {
			tokMin.Limit = n
			if r, ok2 := CaseInsensitiveGet(headers, "anthropic-ratelimit-tokens-remaining"); ok2 {
				if rn, m := parseIntOrZero(r); m != "" {
					debugMsgs = append(debugMsgs, m)
					tokMin = RateLimitBucket{CapturedAt: now}
				} else {
					tokMin.Remaining = rn
					if rs, ok3 := CaseInsensitiveGet(headers, "anthropic-ratelimit-tokens-reset"); ok3 {
						if secs, m := parseISO8601ResetSeconds(rs, now); m != "" {
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

	// Hour 버킷: Anthropic은 별도 시간 단위 헤더 없음 → zero-value
	state.RequestsHour = RateLimitBucket{CapturedAt: now}
	state.TokensHour = RateLimitBucket{CapturedAt: now}

	return state, debugMsgs
}
