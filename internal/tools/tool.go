package tools

import (
	"context"
	"encoding/json"
)

// Tool은 모델이 호출 가능한 단일 기능 단위이다.
// REQ-TOOLS-001: Name은 Registry의 키, Schema는 모델에게 노출되는 입력 계약.
//
// @MX:ANCHOR: [AUTO] Tool 실행 인프라의 핵심 인터페이스
// @MX:REASON: SPEC-GOOSE-TOOLS-001 - Registry/Executor/Inventory 모두 이 인터페이스에 의존. fan_in >= 5 예상
type Tool interface {
	// Name은 Registry에서 사용하는 고유 식별자를 반환한다.
	Name() string
	// Schema는 JSON Schema draft 2020-12 형식의 입력 계약을 반환한다.
	Schema() json.RawMessage
	// Scope는 coordinator/worker 가시성 제어 값을 반환한다.
	Scope() Scope
	// Call은 tool을 실행하고 결과를 반환한다.
	Call(ctx context.Context, input json.RawMessage) (ToolResult, error)
}

// ToolResult는 Tool.Call의 반환 타입이다.
type ToolResult struct {
	// Content는 UTF-8 텍스트 또는 base64 데이터이다.
	Content []byte
	// IsError는 에러 여부이다.
	IsError bool
	// Metadata는 exit_code, bytes_read 등 부가 정보이다.
	Metadata map[string]any
}

// ToolDescriptor는 Inventory.ForModel이 반환하는 tool 설명 타입이다.
type ToolDescriptor struct {
	// Name은 canonical tool 이름이다.
	Name string
	// Description은 tool 설명이다.
	Description string
	// Schema는 JSON Schema이다.
	Schema json.RawMessage
	// Scope는 가시성 제어 값이다.
	Scope Scope
	// Source는 tool 출처이다.
	Source Source
	// ServerID는 MCP tool의 server ID이다.
	ServerID string
}
