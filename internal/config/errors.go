// Package config는 goosed 계층형 설정 로더를 제공한다.
// SPEC-GOOSE-CONFIG-001 — 계층형 설정 로더
package config

import (
	"fmt"
	"strings"
)

// ErrSyntax는 YAML 구문 오류를 나타내는 sentinel 에러다.
// errors.Is()로 비교 가능하다.
var ErrSyntax = fmt.Errorf("config syntax error")

// ErrStrictUnknown은 strict 모드에서 알 수 없는 키 발견 시 반환되는 에러다.
var ErrStrictUnknown = fmt.Errorf("unknown keys in strict mode")

// ConfigError는 설정 로드 실패를 상세히 기술하는 구조체다.
// REQ-CFG-009: 파일 경로, 라인 번호, 컬럼 포함
type ConfigError struct {
	// File은 오류가 발생한 설정 파일 경로다.
	File string
	// Line은 오류 발생 라인 번호다 (yaml.v3가 보고하지 않으면 0).
	Line int
	// Column은 오류 발생 컬럼 번호다 (yaml.v3가 보고하지 않으면 0).
	Column int
	// Msg는 오류 상세 메시지다.
	Msg string
	// Underlying은 원래 에러다.
	Underlying error
}

func (e *ConfigError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("config error in %s:%d:%d: %s", e.File, e.Line, e.Column, e.Msg)
	}
	return fmt.Sprintf("config error in %s: %s", e.File, e.Msg)
}

func (e *ConfigError) Is(target error) bool {
	return target == ErrSyntax
}

func (e *ConfigError) Unwrap() error {
	return e.Underlying
}

// ErrInvalidField는 타입 불일치 또는 범위 검증 실패를 나타낸다.
// REQ-CFG-010: 필드 경로와 기대 타입 포함
type ErrInvalidField struct {
	// Path는 점(.)으로 구분된 필드 경로다. 예: "transport.grpc_port"
	Path string
	// Msg는 오류 상세 메시지다.
	Msg string
	// Expected는 기대되는 타입이다.
	Expected string
	// Got은 실제 받은 타입이다.
	Got string
}

func (e ErrInvalidField) Error() string {
	if e.Expected != "" && e.Got != "" {
		return fmt.Sprintf("invalid field %s: expected %s, got %s", e.Path, e.Expected, e.Got)
	}
	return fmt.Sprintf("invalid field %s: %s", e.Path, e.Msg)
}

// StrictUnknownError는 strict 모드에서 알 수 없는 키 발견 시 반환되는 에러다.
// REQ-CFG-014: 모든 알 수 없는 키 목록 포함
type StrictUnknownError struct {
	// Keys는 발견된 알 수 없는 키 목록이다.
	Keys []string
}

func (e *StrictUnknownError) Error() string {
	return fmt.Sprintf("unknown keys in strict mode: [%s]", strings.Join(e.Keys, ", "))
}

func (e *StrictUnknownError) Is(target error) bool {
	return target == ErrStrictUnknown
}
