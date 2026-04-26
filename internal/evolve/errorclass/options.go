// Package errorclass — ClassifierOptions, FlagProfile, ProviderPattern 타입 정의
package errorclass

import "regexp"

// FlagProfile은 FailoverReason별 4개 회복 신호를 담는다.
// OverrideFlags에서 기본값을 대체하거나, DefaultFlags()로 기본값을 조회한다.
type FlagProfile struct {
	Retryable              bool
	ShouldCompress         bool
	ShouldRotateCredential bool
	ShouldFallback         bool
}

// ProviderPattern은 특정 provider에서만 적용되는 regex 매칭 규칙이다.
// Pattern이 nil이면 stage 1 평가 중 패닉이 발생할 수 있으므로 recover()로 보호된다.
type ProviderPattern struct {
	Provider string
	Pattern  *regexp.Regexp
	Reason   FailoverReason
}

// ClassifierOptions는 New()에 전달되는 선택적 설정이다.
type ClassifierOptions struct {
	// ExtraPatterns는 stage 1 실행 시 built-in 패턴보다 먼저 검사된다 (REQ-023).
	ExtraPatterns []ProviderPattern
	// OverrideFlags는 특정 reason의 기본 플래그를 배포별 정책으로 대체한다 (REQ-024).
	OverrideFlags map[FailoverReason]FlagProfile
}
