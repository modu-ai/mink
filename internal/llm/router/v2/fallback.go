// Package v2 — fallback.go: ERROR-CLASS-001 14 FailoverReason 분기 + chain 실행.
//
// FallbackExecutor 는 사용자 정의 fallback chain 을 순회하며 각 후보의 호출
// 결과를 ErrorClassifier 로 분류하고, 14 reason 을 11 next + 3 stop 으로
// 분기한다. exclude list 가 설정되면 chain 진입 전 silent skip 한다.
//
// 분류 정책 (spec.md §4.4 + plan.md §1 Phase 3):
//   - STOP_CHAIN (3): ContextOverflow, FormatError, PayloadTooLarge —
//     다음 후보로도 같은 입력이라 회복 불가능, 즉시 중단.
//   - NEXT_CANDIDATE (11): 그 외 모든 reason — 다음 후보 시도 가능.
//
// SPEC: SPEC-GOOSE-LLM-ROUTING-V2-001
// REQ: REQ-RV2-005 / REQ-RV2-011 / REQ-RV2-012 / REQ-RV2-013 / REQ-RV2-014
package v2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modu-ai/mink/internal/evolve/errorclass"
)

// ErrEmptyChain 은 Execute 가 빈 chain 으로 호출되었을 때 반환되는 sentinel.
var ErrEmptyChain = errors.New("v2: empty fallback chain")

// ErrAllExcluded 는 chain 의 모든 후보가 ExcludedProviders 에 포함되어
// 시도 가능한 후보가 한 명도 남지 않을 때 반환되는 sentinel (REQ-RV2-012).
var ErrAllExcluded = errors.New("v2: all chain candidates excluded")

// CallFn 은 FallbackExecutor.Execute 가 후보별로 호출하는 함수형 패턴이다.
// caller 가 LLM HTTP 호출, mock provider, fake stub 등 다양하게 대체 가능.
//
// 시그니처:
//   - ctx: caller 의 cancellation context (chain 단위로 공유)
//   - ref: 호출 대상 provider/model
//   - 반환: any (성공 응답 — caller 가 타입 assertion) + error
type CallFn func(ctx context.Context, ref ProviderRef) (any, error)

// Attempt 는 chain 의 단일 후보 시도 결과 기록이다.
// LastAttempts() 로 caller 가 후처리 (progress.md trace, hook) 시 사용.
type Attempt struct {
	// Provider 는 호출된 provider id.
	Provider string
	// Model 은 호출된 model id.
	Model string
	// Success 는 호출이 nil error 로 종료되었는지 여부.
	Success bool
	// Reason 은 실패 시 ErrorClassifier 로 분류된 FailoverReason.
	// Success=true 면 errorclass.Unknown.
	Reason errorclass.FailoverReason
	// Err 은 원본 error (Success=true 면 nil).
	Err error
}

// FallbackError 는 chain 종료 시 (success 외 모든 경로) 반환되는 wrapped
// error 이다. Stopped=true 면 stop chain reason 트리거, false 면 chain
// exhaustion 이다.
type FallbackError struct {
	// Stopped 는 stop chain reason 트리거 여부.
	Stopped bool
	// LastReason 은 마지막 시도의 분류 reason.
	LastReason errorclass.FailoverReason
	// Attempts 는 chain 의 모든 시도 기록 (성공/실패 포함).
	Attempts []Attempt
}

// Error 는 error 인터페이스 구현. 마지막 attempt 의 reason + provider 를
// 사람이 읽는 형식으로 요약한다.
func (e *FallbackError) Error() string {
	if e == nil {
		return "v2: fallback error <nil>"
	}
	if len(e.Attempts) == 0 {
		return "v2: fallback error: no attempts"
	}
	verb := "exhausted"
	if e.Stopped {
		verb = "stopped"
	}
	last := e.Attempts[len(e.Attempts)-1]
	return fmt.Sprintf("v2: fallback chain %s after %d attempt(s); last=%s/%s reason=%s",
		verb, len(e.Attempts), last.Provider, last.Model, e.LastReason)
}

// Unwrap 는 errors.Is/As 와의 호환성을 위해 마지막 attempt 의 raw error 를
// 노출한다. 여러 attempt 가 있을 때는 join 하지 않고 마지막 것만 — 호출자가
// 모든 시도 기록을 보려면 Attempts 필드를 직접 조회.
func (e *FallbackError) Unwrap() error {
	if e == nil || len(e.Attempts) == 0 {
		return nil
	}
	return e.Attempts[len(e.Attempts)-1].Err
}

// FallbackExecutor 는 RoutingPolicy 의 FallbackChain 을 순회 실행하는
// 진입점이다. 동일 인스턴스가 여러 Execute 호출에 재사용 가능하지만 동시
// 호출 시 LastAttempts() 가 마지막 호출의 결과만 반영한다.
//
// @MX:ANCHOR: [AUTO] RouterV2 의 fallback chain 실행 단일 진입점
// @MX:REASON: SPEC §4.4 의 14 FailoverReason 분기 정책 source-of-truth, RouterV2 + 후속 SPEC 이 의존 (fan_in >= 3 예상)
type FallbackExecutor struct {
	classifier  errorclass.Classifier
	excluded    map[string]struct{}
	lastAttempt []Attempt
}

// NewFallbackExecutor 는 ErrorClassifier 를 주입한 executor 를 생성한다.
// classifier 가 nil 이면 모든 에러가 Unknown 으로 분류된다 (테스트 편의).
func NewFallbackExecutor(cls errorclass.Classifier) *FallbackExecutor {
	return &FallbackExecutor{classifier: cls}
}

// SetExcluded 는 RoutingPolicy.ExcludedProviders 를 executor 에 적용한다.
// chain 진입 시 silent skip 대상이 된다 (REQ-RV2-012).
//
// 빈 slice 또는 nil 호출 시 exclude 가 해제된다 (re-use 시 깨끗한 상태).
func (e *FallbackExecutor) SetExcluded(providers []string) {
	if len(providers) == 0 {
		e.excluded = nil
		return
	}
	e.excluded = make(map[string]struct{}, len(providers))
	for _, p := range providers {
		e.excluded[p] = struct{}{}
	}
}

// LastAttempts 는 가장 최근 Execute 호출의 시도 기록을 반환한다.
// caller 가 progress.md 에 trace 를 기록할 때 사용 (REQ-RV2-011).
//
// 반환된 slice 는 executor 내부 상태의 사본이 아니다 — caller 가 수정하면
// 다음 Execute 호출 전까지 같은 값이 보인다. 보수적으로는 사본을 반환해야
// 하지만, 호출 사이트가 모두 read-only 라 비용 낭비를 피한다.
func (e *FallbackExecutor) LastAttempts() []Attempt {
	return e.lastAttempt
}

// Execute 는 chain 을 순회하며 각 후보를 fn 으로 호출하고 14 FailoverReason
// 분기 정책에 따라 다음 후보 시도 / chain 중단을 결정한다.
//
// 반환:
//   - 성공한 후보의 fn 반환값 + nil error
//   - 빈 chain → ErrEmptyChain
//   - 모든 후보 excluded → ErrAllExcluded
//   - stop chain reason 트리거 → *FallbackError{Stopped: true}
//   - chain exhaustion → *FallbackError{Stopped: false}
//   - ctx 취소 → ctx.Err() wrap
//
// 모든 분기에서 LastAttempts() 가 시도 기록을 보존한다.
func (e *FallbackExecutor) Execute(ctx context.Context, chain []ProviderRef, fn CallFn) (any, error) {
	e.lastAttempt = nil
	if len(chain) == 0 {
		return nil, ErrEmptyChain
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("v2: fallback ctx canceled: %w", err)
	}

	attempts := make([]Attempt, 0, len(chain))
	for _, ref := range chain {
		if e.isExcluded(ref.Provider) {
			continue
		}
		if err := ctx.Err(); err != nil {
			e.lastAttempt = attempts
			return nil, fmt.Errorf("v2: fallback ctx canceled mid-chain: %w", err)
		}

		result, err := fn(ctx, ref)
		if err == nil {
			attempts = append(attempts, Attempt{
				Provider: ref.Provider,
				Model:    ref.Model,
				Success:  true,
				Reason:   errorclass.Unknown,
			})
			e.lastAttempt = attempts
			return result, nil
		}

		reason := e.classify(ctx, err, ref)
		attempts = append(attempts, Attempt{
			Provider: ref.Provider,
			Model:    ref.Model,
			Success:  false,
			Reason:   reason,
			Err:      err,
		})

		if isStopChainReason(reason) {
			e.lastAttempt = attempts
			return nil, &FallbackError{
				Stopped:    true,
				LastReason: reason,
				Attempts:   attempts,
			}
		}
		// next candidate 분기는 별도 처리 없이 loop 계속.
	}

	e.lastAttempt = attempts
	if len(attempts) == 0 {
		// chain 전체가 excluded — 한 번도 fn 호출 안 됨.
		return nil, fmt.Errorf("%w: %s", ErrAllExcluded, joinProviders(chain))
	}
	last := attempts[len(attempts)-1]
	return nil, &FallbackError{
		Stopped:    false,
		LastReason: last.Reason,
		Attempts:   attempts,
	}
}

// isExcluded 는 provider id 가 SetExcluded 로 등록되었는지 검사한다.
func (e *FallbackExecutor) isExcluded(provider string) bool {
	if e.excluded == nil {
		return false
	}
	_, ok := e.excluded[provider]
	return ok
}

// classify 는 classifier 가 주입된 경우 호출하고, 아니면 Unknown 을 반환한다.
func (e *FallbackExecutor) classify(ctx context.Context, err error, ref ProviderRef) errorclass.FailoverReason {
	if e.classifier == nil {
		return errorclass.Unknown
	}
	res := e.classifier.Classify(ctx, err, errorclass.ErrorMeta{
		Provider: ref.Provider,
		Model:    ref.Model,
		RawError: err,
	})
	return res.Reason
}

// isStopChainReason 은 spec.md §4.4 REQ-RV2-013 의 3 stop reason 매핑이다.
// ERROR-CLASS-001 14 enum 중 ContextOverflow / FormatError / PayloadTooLarge
// 만 chain 즉시 중단 — 다음 후보로도 같은 입력이라 회복 불가능.
func isStopChainReason(r errorclass.FailoverReason) bool {
	switch r {
	case errorclass.ContextOverflow,
		errorclass.FormatError,
		errorclass.PayloadTooLarge:
		return true
	default:
		return false
	}
}

// joinProviders 는 ErrAllExcluded 에러 메시지에 chain 의 provider id 들을
// 보고할 때 사용한다.
func joinProviders(chain []ProviderRef) string {
	if len(chain) == 0 {
		return ""
	}
	names := make([]string, len(chain))
	for i, r := range chain {
		names[i] = r.Provider
	}
	return strings.Join(names, ",")
}
