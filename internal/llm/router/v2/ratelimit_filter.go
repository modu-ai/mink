// Package v2 — ratelimit_filter.go: RATELIMIT-001 reader 추상화 + 후보 필터.
//
// RouterV2 는 RATELIMIT-001 의 구체 구현에 의존하지 않고 RateLimitView
// 인터페이스를 통해 4 bucket usage 를 조회한다. 이는 어댑터 layer 작성
// 비용을 RATELIMIT-001 본체에 부담시키지 않으면서 v2 의 의사결정 트리를
// 독립적으로 테스트 가능하게 한다 (REQ-RV2-009 + plan.md §2 Risks #1).
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-005 / REQ-RV2-006 / REQ-RV2-009
package v2

// RateLimitView 는 provider 의 4-bucket rate-limit usage 를 [0.0, 1.0]
// 범위 비율로 노출하는 read-only 어댑터이다.
//
// bucket 의미:
//   - rpm: requests per minute usage ratio
//   - tpm: tokens per minute usage ratio
//   - rph: requests per hour usage ratio
//   - tph: tokens per hour usage ratio
//
// provider 가 RATELIMIT-001 tracker 에 등록되어 있지 않으면 모든 bucket
// 0.0 을 반환한다 (보수적 — usage 정보 없음 = 여유로 가정).
type RateLimitView interface {
	BucketUsage(provider string) (rpm, tpm, rph, tph float64)
}

// FilterByRateLimit 는 candidates 중 4 bucket 어느 하나라도 threshold
// 이상인 provider 를 제외한 새 slice 를 반환한다 (REQ-RV2-009).
//
// 인자:
//   - candidates: 입력 후보 (수정되지 않음, 새 slice 반환)
//   - view: RATELIMIT-001 reader 어댑터 (nil 이면 candidates 그대로 반환)
//   - threshold: [0.0, 1.0] 범위 임계 — 보통 DefaultRateLimitThreshold (0.80)
//
// threshold 는 RoutingPolicy.RateLimitThreshold 에서 전달되며 LoadPolicy
// 가 [0.0, 1.0] 범위를 보장한다 (ErrInvalidThreshold). 따라서 본 함수는
// threshold 의 범위 검증을 다시 하지 않는다.
//
// 비교 연산자는 ">=" 이다. 즉 정확히 threshold 인 bucket 도 제외된다 —
// 80% 도달 = 곧 100% 라는 보수적 회피 정책 (REQ-RV2-009 의 명시).
//
// nil 또는 빈 candidates 는 빈 non-nil slice 를 반환한다 (caller 가
// len() 으로 검사할 때 nil-vs-empty 구분 부담 제거).
func FilterByRateLimit(candidates []ProviderRef, view RateLimitView, threshold float64) []ProviderRef {
	if view == nil {
		out := make([]ProviderRef, len(candidates))
		copy(out, candidates)
		return out
	}
	out := make([]ProviderRef, 0, len(candidates))
	for _, c := range candidates {
		rpm, tpm, rph, tph := view.BucketUsage(c.Provider)
		if rpm >= threshold || tpm >= threshold || rph >= threshold || tph >= threshold {
			continue
		}
		out = append(out, c)
	}
	return out
}
