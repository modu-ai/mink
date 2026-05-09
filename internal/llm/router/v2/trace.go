// Package v2 — trace.go: RoutingReason v2 prefix 빌더.
//
// spec.md §6.4 의 7 가지 형식을 단일 source-of-truth 로 모은다. hook
// 소비자 (logging, metrics, progress.md trace) 가 본 builder 출력에
// 의존하므로 형식 회귀는 즉시 trace_test.go 에서 잡힌다.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-006 / REQ-RV2-011 / REQ-RV2-014
package v2

import (
	"fmt"

	"github.com/modu-ai/goose/internal/evolve/errorclass"
)

// TraceV1Simple 은 v2 정책 비활성 + v1 simple 판정 시 사용한다.
// v1 의 "simple_turn" 와는 별개로 v2 decorator 가 통과 결정을 명시할 때.
func TraceV1Simple() string {
	return "v1:simple"
}

// TraceV1Complex 은 v2 정책 비활성 + v1 complex 판정 시 사용한다.
func TraceV1Complex() string {
	return "v1:complex"
}

// TraceV2Policy 는 v2 정책 적용으로 v1 결정을 변경/유지했을 때 사용한다.
//
// 형식: "v2:policy_<mode>_<provider>"
// 예시: "v2:policy_prefer_cheap_groq", "v2:policy_always_specific_openai"
func TraceV2Policy(mode PolicyMode, provider string) string {
	return fmt.Sprintf("v2:policy_%s_%s", mode.String(), provider)
}

// TraceV2Capability 는 required_capabilities 필터 적용 후의 후보 수를
// 보고할 때 사용한다.
//
// 형식: "v2:capability_<cap>_required_<count>_candidates"
// 예시: "v2:capability_vision_required_3_candidates"
func TraceV2Capability(cap Capability, count int) string {
	return fmt.Sprintf("v2:capability_%s_required_%d_candidates", cap, count)
}

// TraceV2RateLimit 는 80% 임계 회피로 후보를 제외한 사실을 보고할 때 사용한다.
//
// 형식: "v2:rate_limit_avoid_<provider>_<bucket>_<usage>"
// usage 는 소수점 둘째 자리에서 반올림 (부동소수 노이즈 노출 방지).
// 예시: "v2:rate_limit_avoid_anthropic_rpm_0.85"
func TraceV2RateLimit(provider, bucket string, usage float64) string {
	return fmt.Sprintf("v2:rate_limit_avoid_%s_%s_%.2f", provider, bucket, usage)
}

// TraceV2FallbackStep 는 chain n 번째 시도의 reason 분류를 보고할 때 사용한다.
//
// 형식: "v2:fallback_chain_step_<n>_<reason>"
// step 은 1-based, reason 은 errorclass.FailoverReason.String() 의 snake_case.
// 예시: "v2:fallback_chain_step_1_rate_limit"
func TraceV2FallbackStep(step int, reason errorclass.FailoverReason) string {
	return fmt.Sprintf("v2:fallback_chain_step_%d_%s", step, reason)
}

// TraceV2FallbackExhausted 는 모든 fallback 후보 실패로 v1 결정을 silent
// recovery 했을 때 사용한다 (REQ-RV2-014).
//
// 형식: "v2:fallback_exhausted_recover_v1" (고정 상수 — caller 가 변형 금지)
func TraceV2FallbackExhausted() string {
	return "v2:fallback_exhausted_recover_v1"
}
