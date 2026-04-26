// Package errorclass — Provider 특화 패턴 + message regex (stage 1, 4)
package errorclass

import (
	"regexp"
	"strings"
)

// BuiltinProviderPatterns는 Hermes error_classifier.py §5 기반.
// built-in 지원 provider: anthropic, openai (REQ-018 v0.1.1).
// 추가 provider는 ClassifierOptions.ExtraPatterns로 주입.
//
// @MX:NOTE: [AUTO] stage 1 매칭 시 provider 이름 비교는 소문자 정규화 후 수행
var BuiltinProviderPatterns = []ProviderPattern{
	{
		Provider: "anthropic",
		Pattern:  regexp.MustCompile(`thinking_signature`),
		Reason:   ThinkingSignature,
	},
	{
		Provider: "anthropic",
		Pattern:  regexp.MustCompile(`long_context_tier`),
		Reason:   ContextOverflow,
	},
	{
		Provider: "openai",
		Pattern:  regexp.MustCompile(`insufficient_quota`),
		Reason:   Billing,
	},
	{
		Provider: "openai",
		Pattern:  regexp.MustCompile(`context_length_exceeded`),
		Reason:   ContextOverflow,
	},
}

// messagePatterns는 stage 4에서 error message 내용 기반 분류에 사용된다.
// 순서가 중요: 더 구체적인 패턴을 먼저 배치.
var messagePatterns = []struct {
	pattern *regexp.Regexp
	reason  FailoverReason
}{
	// Context overflow (400 ambiguous case를 처리)
	{regexp.MustCompile(`(?i)context.*length.*exceed`), ContextOverflow},
	{regexp.MustCompile(`(?i)maximum.*context`), ContextOverflow},
	{regexp.MustCompile(`(?i)token.*limit`), ContextOverflow},
	{regexp.MustCompile(`(?i)too many tokens`), ContextOverflow},
	{regexp.MustCompile(`(?i)context.*window`), ContextOverflow},
	{regexp.MustCompile(`(?i)prompt.*too.*long`), ContextOverflow},
	// Payload too large
	{regexp.MustCompile(`(?i)payload.*too.*large`), PayloadTooLarge},
	{regexp.MustCompile(`(?i)request.*body.*too.*large`), PayloadTooLarge},
	{regexp.MustCompile(`(?i)request.*entity.*too.*large`), PayloadTooLarge},
	// Billing
	{regexp.MustCompile(`(?i)insufficient.?quota`), Billing},
	{regexp.MustCompile(`(?i)credit.*exhausted`), Billing},
	{regexp.MustCompile(`(?i)billing.*hard.*limit`), Billing},
	{regexp.MustCompile(`(?i)exceeded.*current.*quota`), Billing},
	// Model not found
	{regexp.MustCompile(`(?i)model.*not.*found`), ModelNotFound},
	{regexp.MustCompile(`(?i)no.*such.*model`), ModelNotFound},
	{regexp.MustCompile(`(?i)invalid.*model`), ModelNotFound},
	{regexp.MustCompile(`(?i)model.*does.*not.*exist`), ModelNotFound},
	// Auth permanent (permission/forbidden)
	{regexp.MustCompile(`(?i)permission.*denied`), AuthPermanent},
	{regexp.MustCompile(`(?i)forbidden`), AuthPermanent},
	{regexp.MustCompile(`(?i)not.*allowed`), AuthPermanent},
	// Rate limit (message-only, no status)
	{regexp.MustCompile(`(?i)rate.?limit`), RateLimit},
	{regexp.MustCompile(`(?i)too.*many.*requests`), RateLimit},
}

// matchProviderPatterns는 patterns 슬라이스에서 provider+message 매칭을 수행한다.
//
// nil Pattern은 즉시 패닉을 유발한다 — 호출자(Classify)의 defer recover()가 이를 포획한다.
// 이는 REQ-019의 의도적 설계: ExtraPatterns에 잘못된 nil 패턴이 주입된 경우
// Unknown 반환으로 안전하게 처리한다.
func matchProviderPatterns(patterns []ProviderPattern, provider, msg string) (FailoverReason, bool) {
	providerLow := strings.ToLower(provider)
	for _, p := range patterns {
		if strings.ToLower(p.Provider) != providerLow {
			continue
		}
		// p.Pattern이 nil이면 여기서 nil pointer panic — Classify의 recover()가 포획
		if p.Pattern.MatchString(msg) {
			return p.Reason, true
		}
	}
	return Unknown, false
}

// matchMessageRegex는 stage 4에서 error message에 대한 regex 분류를 수행한다.
func matchMessageRegex(msg string) (FailoverReason, bool) {
	for _, mp := range messagePatterns {
		if mp.pattern.MatchString(msg) {
			return mp.reason, true
		}
	}
	return Unknown, false
}
