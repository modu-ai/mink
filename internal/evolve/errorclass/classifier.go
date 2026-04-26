// Package errorclass — Classifier 인터페이스 + 5단계 파이프라인 구현
//
// 5단계 파이프라인 (REQ-003 strict order):
//  1. Provider-specific (ExtraPatterns + BuiltinProviderPatterns)
//  2. HTTP status (with stage 4 override check per REQ-022)
//  3. Error code (body.error.code 기반 — 현재 미사용, 확장점)
//  4. Message regex
//  5. Transport heuristic
//
// 패닉 방어: defer recover() → Unknown 반환 (REQ-019)
// nil error 처리: 즉시 Unknown 반환 (REQ-017)
package errorclass

import (
	"context"
	"strings"
)

// ErrorMeta는 Classify() 호출 시 함께 전달되는 컨텍스트 정보다.
// read-only: Classify()는 이 구조체를 수정하지 않는다 (REQ-020).
type ErrorMeta struct {
	Provider      string
	Model         string
	StatusCode    int
	ApproxTokens  int
	ContextLength int
	MessageCount  int
	RawError      error
}

// ClassifiedError는 Classify()의 반환값이다.
// MatchedBy는 어느 stage에서 분류됐는지 기록 (디버깅/tracing용).
type ClassifiedError struct {
	Reason                 FailoverReason
	StatusCode             int
	Retryable              bool
	ShouldCompress         bool
	ShouldRotateCredential bool
	ShouldFallback         bool
	Message                string // 사람이 읽는 요약
	MatchedBy              string // "stage1_provider"|"stage2_http"|"stage3_code"|"stage4_message"|"stage5_transport"|"fallback"
	RawError               error  // errors.Unwrap chain 보존 (REQ-004)
}

// Classifier는 LLM 오류를 분류하는 인터페이스다.
//
// @MX:ANCHOR: [AUTO] ADAPTER-001, ROUTER-001, CREDPOOL-001이 의존하는 공개 API
// @MX:REASON: fan_in >= 4 (adapter, router, credpool, test suite)
type Classifier interface {
	Classify(ctx context.Context, err error, meta ErrorMeta) ClassifiedError
}

type defaultClassifier struct {
	opts ClassifierOptions
}

// New는 ClassifierOptions로 설정된 Classifier를 생성한다.
func New(opts ClassifierOptions) Classifier {
	return &defaultClassifier{opts: opts}
}

// Classify는 5단계 파이프라인을 실행하여 오류를 분류한다.
//
// @MX:WARN: [AUTO] defer+recover로 패닉을 포획 — 실제 버그가 숨겨질 수 있음
// @MX:REASON: REQ-019 패닉 방어 필수; recover 발생 시 zap error 레벨 로그 권장
func (c *defaultClassifier) Classify(_ context.Context, err error, meta ErrorMeta) (result ClassifiedError) {
	// REQ-017: nil error 즉시 처리
	if err == nil {
		return ClassifiedError{
			Reason:    Unknown,
			Retryable: false,
			Message:   "nil error",
			MatchedBy: "nil_guard",
		}
	}

	// REQ-019: 패닉 방어
	defer func() {
		if r := recover(); r != nil {
			result = ClassifiedError{
				Reason:    Unknown,
				Retryable: false,
				Message:   "classification panic recovered",
				MatchedBy: "panic_guard",
				RawError:  err,
			}
		}
	}()

	msg := err.Error()

	// Stage 1: Provider-specific
	// ExtraPatterns → BuiltinProviderPatterns 순서 (REQ-023)
	if meta.Provider != "" {
		allPatterns := make([]ProviderPattern, 0, len(c.opts.ExtraPatterns)+len(BuiltinProviderPatterns))
		allPatterns = append(allPatterns, c.opts.ExtraPatterns...)
		allPatterns = append(allPatterns, BuiltinProviderPatterns...)

		if reason, ok := matchProviderPatterns(allPatterns, meta.Provider, msg); ok {
			return c.build(reason, "stage1_provider", err, meta)
		}
	}

	// Stage 2: HTTP status (REQ-022 override 포함)
	if reason, ok := matchHTTPStatus(meta.StatusCode); ok {
		// REQ-022: stage 4 regex가 더 구체적인 reason을 제시하면 override
		if overrideReason, overrideOK := matchMessageRegex(msg); overrideOK && overrideReason != reason {
			return c.build(overrideReason, "stage4_message", err, meta)
		}
		return c.build(reason, "stage2_http", err, meta)
	}

	// Stage 3: Error code (확장점 — 현재 구현에서는 메시지 기반으로 처리)
	// body.error.code 파싱은 ADAPTER-001이 meta에 주입하는 방식으로 확장 예정.
	// 현재는 message에 포함된 코드 문자열을 stage 4에서 처리.

	// Stage 4: Message regex
	if reason, ok := matchMessageRegex(msg); ok {
		return c.build(reason, "stage4_message", err, meta)
	}

	// Stage 5: Transport heuristic
	if reason, ok := matchTransport(err, meta); ok {
		return c.build(reason, "stage5_transport", err, meta)
	}

	// Fallback: Unknown (retryable=true — 기본적으로 한 번은 시도)
	return c.build(Unknown, "fallback", err, meta)
}

// build는 reason + matchedBy + err를 조합하여 ClassifiedError를 생성한다.
// OverrideFlags가 있으면 기본 플래그를 대체한다 (REQ-024).
// meta는 수정하지 않는다 (REQ-020).
func (c *defaultClassifier) build(reason FailoverReason, matchedBy string, err error, meta ErrorMeta) ClassifiedError {
	flags := DefaultFlags(reason)

	// REQ-024: OverrideFlags 적용
	if c.opts.OverrideFlags != nil {
		if override, ok := c.opts.OverrideFlags[reason]; ok {
			flags = override
		}
	}

	return ClassifiedError{
		Reason:                 reason,
		StatusCode:             meta.StatusCode,
		Retryable:              flags.Retryable,
		ShouldCompress:         flags.ShouldCompress,
		ShouldRotateCredential: flags.ShouldRotateCredential,
		ShouldFallback:         flags.ShouldFallback,
		Message:                buildMessage(reason, err),
		MatchedBy:              matchedBy,
		RawError:               err, // REQ-004: unwrap chain 보존
	}
}

// buildMessage는 reason과 원본 error로부터 사람이 읽는 요약 메시지를 만든다.
func buildMessage(reason FailoverReason, err error) string {
	if err == nil {
		return "nil error"
	}
	return reason.String() + ": " + truncateMessage(err.Error(), 200)
}

// truncateMessage는 메시지를 maxLen 바이트로 잘라낸다.
// 사용자 입력이 오류 메시지에 포함될 수 있으므로 크기를 제한한다 (보안 §6.7).
func truncateMessage(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
