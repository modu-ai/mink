package mcp

import (
	"context"
	"fmt"

	"github.com/modu-ai/goose/internal/mcp/transport"
	"go.uber.org/zap"
)

// transportAdapter는 transport.Transport를 mcp.Transport로 변환하는 어댑터이다.
// import cycle을 방지하기 위해 두 패키지의 JSON-RPC 타입을 변환한다.
type transportAdapter struct {
	t transport.Transport
}

// SendRequest는 mcp.JSONRPCRequest를 transport.Request로 변환하여 전송한다.
func (a *transportAdapter) SendRequest(ctx context.Context, req JSONRPCRequest) (JSONRPCResponse, error) {
	treq := transport.Request{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Method:  req.Method,
		Params:  req.Params,
	}
	tresp, err := a.t.SendRequest(ctx, treq)
	if err != nil {
		return JSONRPCResponse{}, err
	}

	resp := JSONRPCResponse{
		JSONRPC: tresp.JSONRPC,
		ID:      tresp.ID,
		Result:  tresp.Result,
	}
	if tresp.Error != nil {
		resp.Error = &JSONRPCError{
			Code:    tresp.Error.Code,
			Message: tresp.Error.Message,
			Data:    tresp.Error.Data,
		}
	}
	return resp, nil
}

// Notify는 mcp.JSONRPCNotification을 transport.Notification으로 변환하여 전송한다.
func (a *transportAdapter) Notify(ctx context.Context, msg JSONRPCNotification) error {
	return a.t.Notify(ctx, transport.Notification{
		JSONRPC: msg.JSONRPC,
		Method:  msg.Method,
		Params:  msg.Params,
	})
}

// OnMessage는 핸들러를 transport.Transport에 등록한다. 타입 변환도 수행한다.
func (a *transportAdapter) OnMessage(handler func(JSONRPCMessage)) {
	a.t.OnMessage(func(msg transport.Message) {
		var errObj *JSONRPCError
		if msg.Error != nil {
			errObj = &JSONRPCError{
				Code:    msg.Error.Code,
				Message: msg.Error.Message,
				Data:    msg.Error.Data,
			}
		}
		handler(JSONRPCMessage{
			JSONRPC: msg.JSONRPC,
			ID:      msg.ID,
			Method:  msg.Method,
			Params:  msg.Params,
			Result:  msg.Result,
			Error:   errObj,
		})
	})
}

// Close는 transport를 닫는다.
func (a *transportAdapter) Close() error {
	return a.t.Close()
}

// wrapTransport는 transport.Transport를 mcp.Transport로 래핑한다.
func wrapTransport(t transport.Transport) Transport {
	return &transportAdapter{t: t}
}

// createStdioTransport는 stdio Transport를 생성한다.
func createStdioTransport(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("stdio transport requires Command field")
	}
	t, err := transport.NewStdioTransport(ctx, cfg.Command, cfg.Args, cfg.Env, zap.NewNop())
	if err != nil {
		return nil, err
	}
	return wrapTransport(t), nil
}

// createWebSocketTransport는 WebSocket Transport를 생성한다.
func createWebSocketTransport(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("websocket transport requires URI field")
	}
	var tlsCfg *transport.TLSConfig
	if cfg.TLS != nil {
		tlsCfg = &transport.TLSConfig{Insecure: cfg.TLS.Insecure}
	}
	t, err := transport.NewWebSocketTransport(ctx, cfg.URI, tlsCfg, zap.NewNop())
	if err != nil {
		// transport 패키지의 ErrTLSValidation을 mcp 패키지의 에러로 변환
		if isTransportTLSError(err) {
			return nil, fmt.Errorf("%w: %v", ErrTLSValidation, err)
		}
		return nil, err
	}
	return wrapTransport(t), nil
}

// createSSETransport는 SSE Transport를 생성한다.
func createSSETransport(ctx context.Context, cfg MCPServerConfig) (Transport, error) {
	if cfg.URI == "" {
		return nil, fmt.Errorf("sse transport requires URI field")
	}
	var tlsCfg *transport.TLSConfig
	if cfg.TLS != nil {
		tlsCfg = &transport.TLSConfig{Insecure: cfg.TLS.Insecure}
	}
	t, err := transport.NewSSETransport(ctx, cfg.URI, tlsCfg, zap.NewNop())
	if err != nil {
		return nil, err
	}
	return wrapTransport(t), nil
}

// isTransportTLSError는 에러가 transport.ErrTLSValidation 타입인지 확인한다.
func isTransportTLSError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(transport.TLSValidationError)
	return ok
}
