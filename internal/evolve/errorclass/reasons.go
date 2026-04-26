// Package errorclass — 14 FailoverReason enum + String/Marshal/Unmarshal
package errorclass

import "fmt"

// FailoverReason은 LLM 어댑터 오류의 분류 이유를 나타낸다.
// 각 값은 회복 신호(retryable, should_compress 등)와 1:1 매핑된다.
//
// @MX:ANCHOR: [AUTO] 14 reason enum — AllFailoverReasons()과 defaultFlags 표의 공유 인덱스
// @MX:REASON: SPEC-GOOSE-ERROR-CLASS-001 §6.3 normative source; fan_in >= 5 (classifier, defaults, test, router, credpool)
type FailoverReason int

const (
	// Unknown은 분류 불가 fallback. 기본적으로 한 번은 재시도.
	Unknown FailoverReason = iota // 0
	// Auth는 401 일시적 인증 실패. credential 회전 후 재시도 가능.
	Auth // 1
	// AuthPermanent는 403 또는 key revoked. 회전+fallback.
	AuthPermanent // 2
	// Billing은 402 / insufficient_quota. credential 회전+fallback.
	Billing // 3
	// RateLimit은 429 과부하. credential 회전 후 재시도.
	RateLimit // 4
	// Overloaded는 503/529 서버 과부하. 재시도+fallback.
	Overloaded // 5
	// ServerError는 500/502 내부 서버 오류. 재시도+fallback.
	ServerError // 6
	// ContextOverflow는 400 context_length_exceeded 또는 transport 휴리스틱. 압축 후 재시도.
	ContextOverflow // 7
	// PayloadTooLarge는 413 페이로드 초과. 압축 후 재시도.
	PayloadTooLarge // 8
	// ModelNotFound는 404 모델 없음. fallback.
	ModelNotFound // 9
	// Timeout은 context.DeadlineExceeded 또는 net.Error.Timeout(). 재시도.
	Timeout // 10
	// FormatError는 400 잘못된 JSON / malformed request. 재시도 불가.
	FormatError // 11
	// ThinkingSignature는 Anthropic 특화 protocol 오류. fallback.
	ThinkingSignature // 12
	// TransportError는 connection reset, EOF (bloat 없음). 재시도.
	TransportError // 13
)

// _reasonStrings는 이유 코드의 snake_case 문자열 표. iota 순서와 일치해야 함.
var _reasonStrings = [14]string{
	"unknown",
	"auth",
	"auth_permanent",
	"billing",
	"rate_limit",
	"overloaded",
	"server_error",
	"context_overflow",
	"payload_too_large",
	"model_not_found",
	"timeout",
	"format_error",
	"thinking_signature",
	"transport_error",
}

// _reasonByName은 UnmarshalText를 위한 역방향 조회 맵.
var _reasonByName map[string]FailoverReason

func init() {
	_reasonByName = make(map[string]FailoverReason, len(_reasonStrings))
	for i, s := range _reasonStrings {
		_reasonByName[s] = FailoverReason(i)
	}
}

// String은 reason을 snake_case 문자열로 반환한다.
func (r FailoverReason) String() string {
	if int(r) < 0 || int(r) >= len(_reasonStrings) {
		return fmt.Sprintf("failover_reason_%d", r)
	}
	return _reasonStrings[r]
}

// MarshalText는 encoding.TextMarshaler를 구현한다. JSON/YAML 직렬화용.
func (r FailoverReason) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

// UnmarshalText는 encoding.TextUnmarshaler를 구현한다. JSON/YAML 역직렬화용.
func (r *FailoverReason) UnmarshalText(b []byte) error {
	s := string(b)
	if v, ok := _reasonByName[s]; ok {
		*r = v
		return nil
	}
	return fmt.Errorf("unknown FailoverReason: %q", s)
}

// AllFailoverReasons는 정의된 14개 FailoverReason slice를 반환한다.
// 순서: Unknown, Auth, ..., TransportError (iota 정의 순서).
func AllFailoverReasons() []FailoverReason {
	return []FailoverReason{
		Unknown,
		Auth,
		AuthPermanent,
		Billing,
		RateLimit,
		Overloaded,
		ServerError,
		ContextOverflow,
		PayloadTooLarge,
		ModelNotFound,
		Timeout,
		FormatError,
		ThinkingSignature,
		TransportError,
	}
}
