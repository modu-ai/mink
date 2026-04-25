// Package tools는 Tool 실행 인프라를 제공한다.
// SPEC-GOOSE-TOOLS-001
package tools

import "errors"

var (
	// ErrInvalidSchema는 tool 등록 시 JSON Schema 유효성 검증 실패 시 반환된다.
	// REQ-TOOLS-002
	ErrInvalidSchema = errors.New("invalid schema")

	// ErrDuplicateName은 이미 등록된 이름으로 등록 시도 시 반환된다.
	// REQ-TOOLS-004, REQ-TOOLS-021
	ErrDuplicateName = errors.New("duplicate tool name")

	// ErrReservedName은 built-in 예약어로 MCP tool 등록 시도 시 반환된다.
	// REQ-TOOLS-003
	ErrReservedName = errors.New("reserved tool name")

	// ErrMCPTimeout은 MCP manifest fetch 가 5초 내 완료되지 않을 때 반환된다.
	// REQ-TOOLS-008
	ErrMCPTimeout = errors.New("mcp manifest fetch timeout")

	// ErrRegistryDraining은 Registry가 Draining 상태일 때 Executor.Run 호출 시 반환된다.
	// REQ-TOOLS-011
	ErrRegistryDraining = errors.New("registry draining")
)
