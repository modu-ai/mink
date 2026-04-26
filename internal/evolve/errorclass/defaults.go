// Package errorclass — 14 FailoverReason × 4-flag 기본값 표
//
// REQ-ERRCLASS-002: 각 reason의 기본 FlagProfile을 코드에 명시한다.
// REQ-ERRCLASS-021: retryable=true AND should_fallback=true 동시 설정은
//
//	Overloaded, ServerError 두 reason에서만 허용.
//
// init()에서 불변식을 검증하여 위반 시 panic을 발생시킨다.
package errorclass

// defaultFlagsTable은 14 reason별 4-flag 기본 정책.
// §6.3 normative source를 코드로 반영한다.
//
// @MX:ANCHOR: [AUTO] REQ-021 invariant table — retryable+fallback 조합 제약
// @MX:REASON: 14개 reason × 4 flag = 56 bits of policy; CREDPOOL/ROUTER/CONTEXT가 이 표를 신뢰
var defaultFlagsTable = map[FailoverReason]FlagProfile{
	//             Retryable  Compress  RotateCred  Fallback
	Unknown:           {true, false, false, false},
	Auth:              {true, false, true, false},
	AuthPermanent:     {false, false, true, true},
	Billing:           {false, false, true, true},
	RateLimit:         {true, false, true, false},
	Overloaded:        {true, false, false, true}, // REQ-021 예외
	ServerError:       {true, false, false, true}, // REQ-021 예외
	ContextOverflow:   {true, true, false, false},
	PayloadTooLarge:   {true, true, false, false},
	ModelNotFound:     {false, false, false, true},
	Timeout:           {true, false, false, false},
	FormatError:       {false, false, false, false},
	ThinkingSignature: {false, false, false, true},
	TransportError:    {true, false, false, false},
}

// _allowedBothRetryableAndFallback는 retryable=true AND should_fallback=true
// 조합이 허용되는 reason 집합 (REQ-021).
var _allowedBothRetryableAndFallback = map[FailoverReason]bool{
	Overloaded:  true,
	ServerError: true,
}

func init() {
	// REQ-021 불변식 검증: 위반 시 init()에서 panic → 런타임 시작 실패로 조기 발견.
	for reason, flags := range defaultFlagsTable {
		if flags.Retryable && flags.ShouldFallback {
			if !_allowedBothRetryableAndFallback[reason] {
				panic("errorclass: REQ-021 violation — reason " + reason.String() +
					" has retryable=true AND should_fallback=true but is not in allowed set")
			}
		}
	}
}

// DefaultFlags는 주어진 reason의 기본 4-flag 프로파일을 반환한다.
// reason이 표에 없으면 Unknown의 기본값을 반환한다.
func DefaultFlags(reason FailoverReason) FlagProfile {
	if f, ok := defaultFlagsTable[reason]; ok {
		return f
	}
	return defaultFlagsTable[Unknown]
}
