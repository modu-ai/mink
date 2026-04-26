// Package errorclass — HTTP status code → FailoverReason 매핑 (stage 2)
package errorclass

// matchHTTPStatus는 HTTP 상태 코드를 FailoverReason으로 매핑한다.
// 400은 ambiguous(FormatError/ContextOverflow 구분 불가)이므로 false 반환.
// stage 4 message regex가 400을 처리한다 (REQ-022).
func matchHTTPStatus(status int) (FailoverReason, bool) {
	switch status {
	case 401:
		return Auth, true
	case 402:
		return Billing, true
	case 403:
		return AuthPermanent, true
	case 404:
		return ModelNotFound, true
	case 413:
		return PayloadTooLarge, true
	case 429:
		return RateLimit, true
	case 500, 502:
		return ServerError, true
	case 503, 529:
		return Overloaded, true
	case 400:
		// ambiguous — stage 4 message regex로 위임 (REQ-022)
		return Unknown, false
	}
	return Unknown, false
}
