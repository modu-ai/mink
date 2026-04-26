package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// MCPServer는 goosed를 MCP 서버로 동작하게 하는 빌더이다.
// REQ-MCP-010: 요청 수신 → handler dispatch → 응답 직렬화
// REQ-MCP-016: reserved tool name 거부
//
// @MX:ANCHOR: [AUTO] MCPServer — MCP 서버 빌더 구현체
// @MX:REASON: REQ-MCP-010, REQ-MCP-016 — 서버 빌더 API의 단일 진입점. fan_in >= 3 (test, adapter, client)
type MCPServer struct {
	info      ServerInfo
	tools     map[string]toolEntry
	resources map[string]ResourceHandler
	prompts   map[string]promptEntry
	mu        sync.RWMutex
}

// promptEntry는 prompt 등록 항목이다.
type promptEntry struct {
	args    []PromptArgument
	handler PromptHandler
}

// NewServer는 새 MCPServer를 생성한다.
// REQ-MCP-010
func NewServer(info ServerInfo) *MCPServer {
	return &MCPServer{
		info:      info,
		tools:     make(map[string]toolEntry),
		resources: make(map[string]ResourceHandler),
		prompts:   make(map[string]promptEntry),
	}
}

// Tool은 tool을 서버에 등록한다.
// REQ-MCP-016: '/', ':', '__' 포함 시 ErrReservedToolName 반환
func (s *MCPServer) Tool(name string, schema json.RawMessage, h ToolHandler) (*MCPServer, error) {
	if err := validateToolName(name); err != nil {
		return s, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[name] = toolEntry{schema: schema, handler: h}
	return s, nil
}

// Resource는 resource handler를 등록한다.
func (s *MCPServer) Resource(uri string, h ResourceHandler) *MCPServer {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources[uri] = h
	return s
}

// Prompt는 prompt handler를 등록한다.
func (s *MCPServer) Prompt(name string, args []PromptArgument, h PromptHandler) *MCPServer {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts[name] = promptEntry{args: args, handler: h}
	return s
}

// Serve는 transport를 통해 MCP 서버를 시작한다.
// REQ-MCP-010: (a) 요청 읽기 (b) handler dispatch (c) 응답 직렬화 (d) transport 쓰기
func (s *MCPServer) Serve(ctx context.Context, t Transport) error {
	// 서버는 transport에서 메시지를 수신하여 처리한다.
	// OnMessage 핸들러를 등록하여 incoming 요청을 처리한다.
	errCh := make(chan error, 1)

	t.OnMessage(func(msg JSONRPCMessage) {
		if !msg.IsRequest() {
			return
		}

		go func() {
			resp, err := s.handleRequest(ctx, msg)
			if err != nil {
				resp = JSONRPCResponse{
					JSONRPC: JSONRPCVersion,
					ID:      msg.ID,
					Error: &JSONRPCError{
						Code:    ErrCodeInternal,
						Message: err.Error(),
					},
				}
			}

			// 응답은 transport.Notify로 전송할 수 없으므로 별도 처리가 필요하다.
			// 실제 서버 구현에서는 transport가 응답을 직접 관리한다.
			// 이 구현에서는 응답을 알림 형식으로 전송한다.
			resultBytes, _ := json.Marshal(resp)
			_ = t.Notify(ctx, JSONRPCNotification{
				JSONRPC: JSONRPCVersion,
				Method:  "rpc.response",
				Params:  resultBytes,
			})
		}()
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// handleRequest는 JSON-RPC 요청을 처리하고 응답을 반환한다.
func (s *MCPServer) handleRequest(ctx context.Context, msg JSONRPCMessage) (JSONRPCResponse, error) {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolsCall(ctx, msg)
	case "resources/list":
		return s.handleResourcesList(msg)
	case "prompts/list":
		return s.handlePromptsList(msg)
	default:
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      msg.ID,
			Error: &JSONRPCError{
				Code:    ErrCodeMethodNotFound,
				Message: fmt.Sprintf("method not found: %s", msg.Method),
			},
		}, nil
	}
}

// handleInitialize는 initialize 요청을 처리한다.
func (s *MCPServer) handleInitialize(msg JSONRPCMessage) (JSONRPCResponse, error) {
	result, _ := json.Marshal(map[string]any{
		"protocolVersion": SupportedProtocolVersions[0],
		"serverInfo": map[string]string{
			"name":    s.info.Name,
			"version": s.info.Version,
		},
		"capabilities": map[string]any{
			"tools":     map[string]any{},
			"resources": map[string]any{},
			"prompts":   map[string]any{},
		},
	})
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: msg.ID, Result: result}, nil
}

// handleToolsList는 tools/list 요청을 처리한다.
func (s *MCPServer) handleToolsList(msg JSONRPCMessage) (JSONRPCResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type toolDef struct {
		Name        string          `json:"name"`
		InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	}

	tools := make([]toolDef, 0, len(s.tools))
	for name, entry := range s.tools {
		tools = append(tools, toolDef{Name: name, InputSchema: entry.schema})
	}

	result, _ := json.Marshal(map[string]any{"tools": tools})
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: msg.ID, Result: result}, nil
}

// handleToolsCall은 tools/call 요청을 처리한다.
func (s *MCPServer) handleToolsCall(ctx context.Context, msg JSONRPCMessage) (JSONRPCResponse, error) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion, ID: msg.ID,
			Error: &JSONRPCError{Code: ErrCodeInvalidParams, Message: "invalid params"},
		}, nil
	}

	s.mu.RLock()
	entry, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		return JSONRPCResponse{
			JSONRPC: JSONRPCVersion, ID: msg.ID,
			Error: &JSONRPCError{Code: ErrCodeMethodNotFound, Message: fmt.Sprintf("tool not found: %s", params.Name)},
		}, nil
	}

	output, err := entry.handler(ctx, params.Arguments)
	if err != nil {
		result, _ := json.Marshal(map[string]any{
			"content": []map[string]any{{"type": "text", "text": err.Error()}},
			"isError": true,
		})
		return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: msg.ID, Result: result}, nil
	}

	result, _ := json.Marshal(map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(output)}},
		"isError": false,
	})
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: msg.ID, Result: result}, nil
}

// handleResourcesList는 resources/list 요청을 처리한다.
func (s *MCPServer) handleResourcesList(msg JSONRPCMessage) (JSONRPCResponse, error) {
	s.mu.RLock()
	uris := make([]string, 0, len(s.resources))
	for uri := range s.resources {
		uris = append(uris, uri)
	}
	s.mu.RUnlock()

	result, _ := json.Marshal(map[string]any{"resources": uris})
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: msg.ID, Result: result}, nil
}

// handlePromptsList는 prompts/list 요청을 처리한다.
func (s *MCPServer) handlePromptsList(msg JSONRPCMessage) (JSONRPCResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type promptDef struct {
		Name      string           `json:"name"`
		Arguments []PromptArgument `json:"arguments,omitempty"`
	}

	prompts := make([]promptDef, 0, len(s.prompts))
	for name, entry := range s.prompts {
		prompts = append(prompts, promptDef{Name: name, Arguments: entry.args})
	}

	result, _ := json.Marshal(map[string]any{"prompts": prompts})
	return JSONRPCResponse{JSONRPC: JSONRPCVersion, ID: msg.ID, Result: result}, nil
}

// ToolNames는 등록된 tool 이름 목록을 반환한다 (테스트 전용).
func (s *MCPServer) ToolNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.tools))
	for name := range s.tools {
		names = append(names, name)
	}
	return names
}
