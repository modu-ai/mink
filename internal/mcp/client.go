package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Client는 MCPClient 인터페이스를 구현하는 MCP 클라이언트이다.
//
// @MX:ANCHOR: [AUTO] Client — MCP 클라이언트 구현체
// @MX:REASON: REQ-MCP-001..023 — 모든 MCP 클라이언트 동작의 구현. fan_in >= 3 (adapter, test, registry)
type Client struct {
	sessions sync.Map // map[string]*ServerSession (key = MCPServerConfig.ID)
	logger   *zap.Logger
	// transportFactory는 transport 생성 함수 (테스트 주입 가능)
	transportFactory TransportFactory
}

// TransportFactory는 MCPServerConfig에서 Transport를 생성하는 함수 타입이다.
// 테스트에서 mock transport를 주입할 때 사용한다.
type TransportFactory func(ctx context.Context, cfg MCPServerConfig) (Transport, error)

// NewClient는 새 MCP 클라이언트를 생성한다.
func NewClient(logger *zap.Logger, factory TransportFactory) *Client {
	if logger == nil {
		logger = zap.NewNop()
	}
	if factory == nil {
		factory = defaultTransportFactory
	}
	return &Client{
		logger:           logger,
		transportFactory: factory,
	}
}

// defaultTransportFactory는 기본 transport 생성자이다.
func defaultTransportFactory(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
	switch cfg.Transport {
	case "stdio":
		// import cycle 방지: transport 패키지 직접 참조 대신 인터페이스 사용
		return createStdioTransport(ctx, cfg)
	case "websocket":
		return createWebSocketTransport(ctx, cfg)
	case "sse":
		return createSSETransport(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown transport type: %s", cfg.Transport)
	}
}

// configID는 MCPServerConfig의 ID를 계산한다.
// REQ-MCP-002: Name + Transport + URI 해시
func configID(cfg MCPServerConfig) string {
	if cfg.ID != "" {
		return cfg.ID
	}
	key := cfg.Name + ":" + cfg.Transport + ":" + cfg.URI + ":" + cfg.Command
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", h[:8])
}

// ConnectToServer는 MCP 서버에 연결한다.
// REQ-MCP-002: 동일 ID의 두 번째 호출은 기존 세션 반환 (memoize)
// REQ-MCP-005: stdio transport의 경우 subprocess spawn + initialize 핸드셰이크
// REQ-MCP-018: 지원하지 않는 프로토콜 버전 시 ErrUnsupportedProtocolVersion
//
// @MX:ANCHOR: [AUTO] Client.ConnectToServer — MCP 서버 연결 및 memoize
// @MX:REASON: REQ-MCP-002, REQ-MCP-005 — 연결 생성의 유일한 진입점. fan_in >= 3
func (c *Client) ConnectToServer(ctx context.Context, cfg MCPServerConfig) (*ServerSession, error) {
	id := configID(cfg)
	cfg.ID = id

	// REQ-MCP-002: memoize 확인
	if existing, ok := c.sessions.Load(id); ok {
		session := existing.(*ServerSession)
		if session.GetState() == SessionConnected {
			return session, nil
		}
		// 연결이 끊겼다면 재연결
		c.sessions.Delete(id)
	}

	// 기본 timeout 설정
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = DefaultRequestTimeout
	}

	// Transport 생성
	t, err := c.transportFactory(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create transport: %w", err)
	}

	session := &ServerSession{
		ID:        id,
		Config:    cfg,
		transport: t,
		State:     SessionConnected,
		logger:    c.logger,
	}

	// REQ-MCP-005: initialize 핸드셰이크
	if err := c.initialize(ctx, session); err != nil {
		_ = t.Close()
		return nil, err
	}

	// 세션 저장
	c.sessions.Store(id, session)

	return session, nil
}

// initialize는 MCP initialize 핸드셰이크를 수행한다.
// REQ-MCP-005: (c) initialize 요청 + (d) serverCapabilities 기록
// REQ-MCP-018: 프로토콜 버전 불일치 시 ErrUnsupportedProtocolVersion
func (c *Client) initialize(ctx context.Context, session *ServerSession) error {
	// clientCapabilities 선언
	clientCaps := map[string]any{
		"sampling":     map[string]any{},
		"roots":        map[string]any{"listChanged": true},
		"experimental": map[string]any{},
	}

	params, _ := json.Marshal(map[string]any{
		"protocolVersion": SupportedProtocolVersions[0],
		"clientInfo":      map[string]string{"name": "goose", "version": "0.2.0"},
		"capabilities":    clientCaps,
	})

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  "initialize",
		Params:  params,
	}

	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := session.transport.SendRequest(initCtx, req)
	if err != nil {
		return fmt.Errorf("initialize request: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	// 응답 파싱
	var initResult struct {
		ProtocolVersion string         `json:"protocolVersion"`
		Capabilities    map[string]any `json:"capabilities"`
		ServerInfo      map[string]any `json:"serverInfo"`
	}
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		return fmt.Errorf("parse initialize response: %w", err)
	}

	// REQ-MCP-018: 프로토콜 버전 검증
	if !isProtocolVersionSupported(initResult.ProtocolVersion) {
		return fmt.Errorf("%w: %s", ErrUnsupportedProtocolVersion, initResult.ProtocolVersion)
	}

	// REQ-MCP-005: serverCapabilities 기록
	session.mu.Lock()
	session.ProtocolVersion = initResult.ProtocolVersion
	session.ServerCapabilities = make(map[string]bool)
	for cap := range initResult.Capabilities {
		session.ServerCapabilities[cap] = true
	}
	session.ClientCapabilities = map[string]bool{
		"sampling": true,
		"roots":    true,
	}
	session.mu.Unlock()

	// initialized 알림 전송
	_ = session.transport.Notify(ctx, JSONRPCNotification{
		JSONRPC: JSONRPCVersion,
		Method:  "notifications/initialized",
	})

	c.logger.Info("mcp: session initialized",
		zap.String("sessionID", session.ID),
		zap.String("protocolVersion", session.ProtocolVersion),
	)

	return nil
}

// ListTools는 서버의 tool 목록을 반환한다. 첫 호출 시 wire 요청, 이후 캐시 반환.
// REQ-MCP-006: deferred loading + 캐시
// REQ-MCP-011: 캐시된 결과 1ms 이내 반환
// REQ-MCP-021: tools capability 미선언 시 ErrCapabilityNotSupported
func (c *Client) ListTools(ctx context.Context, s *ServerSession) ([]MCPTool, error) {
	if err := c.checkConnected(s); err != nil {
		return nil, err
	}

	// REQ-MCP-021: capability 확인
	if !s.HasCapability("tools") {
		return nil, ErrCapabilityNotSupported
	}

	s.mu.RLock()
	if s.toolsLoaded {
		tools := make([]MCPTool, len(s.tools))
		copy(tools, s.tools)
		s.mu.RUnlock()
		return tools, nil
	}
	s.mu.RUnlock()

	// 첫 호출: wire 요청
	tools, err := c.fetchTools(ctx, s)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.tools = tools
	s.toolsLoaded = true
	s.mu.Unlock()

	return tools, nil
}

// fetchTools는 wire 요청으로 tool 목록을 가져온다.
func (c *Client) fetchTools(ctx context.Context, s *ServerSession) ([]MCPTool, error) {
	reqCtx, cancel := c.withRequestTimeout(ctx, s.Config)
	defer cancel()

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  "tools/list",
	}

	resp, err := s.transport.SendRequest(reqCtx, req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %v", ErrRequestTimeout, err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tools/list error: %s", resp.Error.Message)
	}

	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse tools/list: %w", err)
	}

	serverName := s.Config.Name
	seen := make(map[string]bool)
	var tools []MCPTool

	for _, t := range result.Tools {
		if seen[t.Name] {
			return nil, fmt.Errorf("%w: %s", ErrDuplicateMCPToolName, t.Name)
		}
		seen[t.Name] = true

		tools = append(tools, MCPTool{
			Name:        namespacedToolName(serverName, t.Name),
			Description: t.Description,
			InputSchema: t.InputSchema,
			ServerID:    s.ID,
		})
	}

	return tools, nil
}

// CallTool은 MCP tool을 호출한다.
// REQ-MCP-021: tools capability 미선언 시 ErrCapabilityNotSupported
// REQ-MCP-022: request-level deadline + $/cancelRequest
func (c *Client) CallTool(ctx context.Context, s *ServerSession, name string, input json.RawMessage) (ToolResult, error) {
	if err := c.checkConnected(s); err != nil {
		return ToolResult{}, err
	}

	// REQ-MCP-021: capability 확인
	if !s.HasCapability("tools") {
		return ToolResult{}, ErrCapabilityNotSupported
	}

	// tool 이름에서 server prefix 제거하여 실제 tool 이름 추출
	rawToolName := rawToolName(name, s.Config.Name)

	reqCtx, cancel := c.withRequestTimeout(ctx, s.Config)
	defer cancel()

	params, _ := json.Marshal(map[string]any{
		"name":      rawToolName,
		"arguments": input,
	})

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  "tools/call",
		Params:  params,
	}

	resp, err := s.transport.SendRequest(reqCtx, req)
	if err != nil {
		if reqCtx.Err() != nil && ctx.Err() == nil {
			// 요청 timeout (외부 ctx는 아직 유효, 내부 reqCtx timeout)
			return ToolResult{}, ErrRequestTimeout
		}
		if ctx.Err() != nil {
			return ToolResult{}, ctx.Err()
		}
		return ToolResult{}, err
	}

	if resp.Error != nil {
		if resp.Error.Code == ErrCodeRequestCancelled {
			return ToolResult{}, ErrRequestTimeout
		}
		return ToolResult{
			IsError: true,
			Content: json.RawMessage(fmt.Sprintf(`{"error":%q}`, resp.Error.Message)),
		}, nil
	}

	return ToolResult{Content: resp.Result}, nil
}

// rawToolName은 네임스페이스가 적용된 tool 이름에서 원본 이름을 추출한다.
func rawToolName(namespacedName, serverName string) string {
	prefix := "mcp__" + serverName + "__"
	if len(namespacedName) > len(prefix) && namespacedName[:len(prefix)] == prefix {
		return namespacedName[len(prefix):]
	}
	return namespacedName
}

// ListResources는 서버의 resource 목록을 반환한다.
// REQ-MCP-021: resources capability 미선언 시 ErrCapabilityNotSupported
func (c *Client) ListResources(ctx context.Context, s *ServerSession) ([]MCPResource, error) {
	if err := c.checkConnected(s); err != nil {
		return nil, err
	}

	// REQ-MCP-021: capability 확인
	if !s.HasCapability("resources") {
		return nil, ErrCapabilityNotSupported
	}

	reqCtx, cancel := c.withRequestTimeout(ctx, s.Config)
	defer cancel()

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  "resources/list",
	}

	resp, err := s.transport.SendRequest(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("resources/list: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("resources/list error: %s", resp.Error.Message)
	}

	var result struct {
		Resources []MCPResource `json:"resources"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse resources/list: %w", err)
	}

	return result.Resources, nil
}

// ReadResource는 URI에 해당하는 resource 내용을 반환한다.
// REQ-MCP-021: resources capability 미선언 시 ErrCapabilityNotSupported
func (c *Client) ReadResource(ctx context.Context, s *ServerSession, uri string) (ResourceContent, error) {
	if err := c.checkConnected(s); err != nil {
		return ResourceContent{}, err
	}

	// REQ-MCP-021: capability 확인
	if !s.HasCapability("resources") {
		return ResourceContent{}, ErrCapabilityNotSupported
	}

	reqCtx, cancel := c.withRequestTimeout(ctx, s.Config)
	defer cancel()

	params, _ := json.Marshal(map[string]string{"uri": uri})
	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  "resources/read",
		Params:  params,
	}

	resp, err := s.transport.SendRequest(reqCtx, req)
	if err != nil {
		return ResourceContent{}, fmt.Errorf("resources/read: %w", err)
	}

	if resp.Error != nil {
		return ResourceContent{}, fmt.Errorf("resources/read error: %s", resp.Error.Message)
	}

	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return ResourceContent{}, fmt.Errorf("parse resources/read: %w", err)
	}

	if len(result.Contents) == 0 {
		return ResourceContent{URI: uri}, nil
	}

	first := result.Contents[0]
	return ResourceContent{
		URI:      first.URI,
		MimeType: first.MimeType,
		Text:     first.Text,
	}, nil
}

// ListPrompts는 서버의 prompt 목록을 반환한다.
// REQ-MCP-021: prompts capability 미선언 시 ErrCapabilityNotSupported
func (c *Client) ListPrompts(ctx context.Context, s *ServerSession) ([]MCPPrompt, error) {
	if err := c.checkConnected(s); err != nil {
		return nil, err
	}

	// REQ-MCP-021: capability 확인
	if !s.HasCapability("prompts") {
		return nil, ErrCapabilityNotSupported
	}

	reqCtx, cancel := c.withRequestTimeout(ctx, s.Config)
	defer cancel()

	req := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		Method:  "prompts/list",
	}

	resp, err := s.transport.SendRequest(reqCtx, req)
	if err != nil {
		return nil, fmt.Errorf("prompts/list: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("prompts/list error: %s", resp.Error.Message)
	}

	var result struct {
		Prompts []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Arguments   []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Required    bool   `json:"required"`
			} `json:"arguments"`
		} `json:"prompts"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("parse prompts/list: %w", err)
	}

	prompts := make([]MCPPrompt, 0, len(result.Prompts))
	for _, p := range result.Prompts {
		args := make([]PromptArgument, 0, len(p.Arguments))
		for _, a := range p.Arguments {
			args = append(args, PromptArgument{
				Name:        a.Name,
				Description: a.Description,
				Required:    a.Required,
			})
		}
		prompts = append(prompts, MCPPrompt{
			Name:        p.Name,
			Description: p.Description,
			Arguments:   args,
		})
	}

	return prompts, nil
}

// Disconnect는 세션을 종료한다.
// REQ-MCP-009, REQ-MCP-023: transport 닫기 + tool registry sync
func (c *Client) Disconnect(s *ServerSession) error {
	c.sessions.Delete(s.ID)
	s.SetState(SessionDisconnected)

	if s.transport != nil {
		return s.transport.Close()
	}
	return nil
}

// InvalidateToolCache는 tool 캐시를 무효화한다.
// REQ-MCP-006: 다음 ListTools 호출 시 wire 요청 수행
func (c *Client) InvalidateToolCache(s *ServerSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.toolsLoaded = false
	s.tools = nil
}

// checkConnected는 세션이 Connected 상태인지 확인한다.
func (c *Client) checkConnected(s *ServerSession) error {
	if s.GetState() != SessionConnected {
		return ErrSessionNotConnected
	}
	return nil
}

// withRequestTimeout은 요청 timeout이 적용된 context를 반환한다.
// REQ-MCP-022: cfg.RequestTimeout (기본 30s)
func (c *Client) withRequestTimeout(ctx context.Context, cfg MCPServerConfig) (context.Context, context.CancelFunc) {
	timeout := cfg.RequestTimeout
	if timeout == 0 {
		timeout = DefaultRequestTimeout
	}
	return context.WithTimeout(ctx, timeout)
}
