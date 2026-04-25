// Package mcp는 MCP 연결 인터페이스 스텁이다.
// SPEC-GOOSE-MCP-001에서 실제 구현된다. 본 패키지는 TOOLS-001이 참조하는 최소 계약만 정의한다.
package mcp

import "context"

// ToolManifest는 MCP server에서 노출하는 단일 tool 설명이다.
type ToolManifest struct {
	// Name은 tool 이름이다.
	Name string
	// Description은 tool 설명이다.
	Description string
	// InputSchema는 JSON Schema 형식의 입력 스키마이다.
	InputSchema map[string]any
}

// ToolCallResult는 MCP tool 호출 결과이다.
type ToolCallResult struct {
	// Content는 결과 내용이다.
	Content []byte
	// IsError는 에러 여부이다.
	IsError bool
}

// Connection은 MCP server 연결 인터페이스이다.
// SPEC-GOOSE-MCP-001에서 구현된다.
type Connection interface {
	// ServerID는 연결된 MCP server의 식별자를 반환한다.
	ServerID() string
	// ListTools는 server가 노출하는 tool 목록을 반환한다.
	ListTools() []ToolManifest
	// FetchToolManifest는 특정 tool의 매니페스트를 가져온다 (lazy fetch).
	FetchToolManifest(ctx context.Context, toolName string) (ToolManifest, error)
	// CallTool은 MCP tool을 실행하고 결과를 반환한다.
	CallTool(ctx context.Context, toolName string, input map[string]any) (ToolCallResult, error)
}

// Manager는 복수 MCP 연결을 관리하는 인터페이스이다.
// SPEC-GOOSE-MCP-001에서 구현된다.
type Manager interface {
	// Connections는 현재 활성 연결 목록을 반환한다.
	Connections() []Connection
	// Subscribe는 ConnectionClosed 이벤트 구독 채널을 반환한다.
	Subscribe() <-chan ConnectionClosedEvent
}

// ConnectionClosedEvent는 MCP 연결 종료 이벤트이다.
type ConnectionClosedEvent struct {
	// ServerID는 종료된 연결의 server ID이다.
	ServerID string
}
